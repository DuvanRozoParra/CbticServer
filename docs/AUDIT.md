# Auditoría de Performance y Escalabilidad — CbticServer

| Campo | Valor |
|---|---|
| Fecha | 2026-06-24 |
| Versión auditada | snapshot actual (commit `481a9b3`) |
| Stack | Go 1.23.2, Fiber v2.52.6, gofiber/contrib/websocket v1.3.4 |
| **Calificación general** | **3 / 10** |

> **📌 Estado actual (post-refactor):** la mayoría de issues críticos y de escalabilidad están resueltos. Ver sección [§8 Estado de resolución](#8-estado-de-resolución) al final.

---

## 1. Resumen ejecutivo

El servidor funciona para una demo con pocos jugadores (≤10) en LAN, pero **no es escalable** y tiene varios puntos de fallo que pueden colgar el servidor completo. Los problemas no son de sintaxis sino de **diseño de concurrencia, aislamiento de red y hot path**.

### Notas por categoría

| Categoría | Nota | Comentario |
|---|---|---|
| Concurrencia / threading | **2/10** | Lock global + writes de red dentro del lock |
| Tolerancia a fallos | **3/10** | Sin recover, panics posibles, sin deadlines |
| Escalabilidad horizontal/salas | **2/10** | Sala única, límite de 10 colores |
| Eficiencia en hot path | **4/10** | Doble marshal, sin pooling, O(N) por frame |
| Operabilidad (logs/shutdown) | **4/10** | Sin graceful shutdown ni structured logging |

---

## 2. 🔴 Problemas CRÍTICOS (pueden tirar el servidor)

### C1. Lock global que serializa todo el servidor
- **Ubicación:** `internal/network/queue/worker.go:27`
- **Detalle:** cada mensaje toma `jg.Mu.Lock()` (exclusivo) durante **todo** el procesamiento. Los 6 workers declarados son inútiles: en la práctica solo procesa uno a la vez.
- **Impacto:** con N jugadores a 60 Hz, el throughput colapsa; la latencia crece linealmente con N.

### C2. Escrituras de red DENTRO del lock global
- **Ubicación:** `internal/network/queue/worker.go:40,45,50,55` → `pkg/handlers.go:132,150`
- **Detalle:** `WriteMessage` es bloqueante y se ejecuta **sosteniendo el lock global**.
- **Impacto:** un solo cliente lento o congelado **bloquea a todos los jugadores y todo el servidor**. Es el bug más grave: DoS accidental.

### C3. Sin WriteDeadline en los sockets
- **Ubicación:** `pkg/handlers.go:132,150,162`
- **Detalle:** ningún `SetWriteDeadline` antes de `WriteMessage`.
- **Impacto:** un write a un cliente muerto puede quedar colgado indefinidamente, exacerbando C2.

### C4. Panic potencial por nil pointer
- **Ubicación:** `internal/core/players/events.go:13`
- **Detalle:** `jg.Players[playerID].Player = currentPlayer`. Si `ConvertToPlayer` falla el parseo, devuelve `nil`; luego `GetAllPlayer` (`handlers.go:40`) hace `*p.Player` → **panic**.
- **Impacto:** sin `recover`, el worker muere para siempre y la capacidad del servidor decrece permanentemente con cada fallo.

### C5. Sin recovery en workers
- **Ubicación:** `internal/network/queue/worker.go:18`
- **Detalle:** el cuerpo del `for` no tiene `defer recover`.
- **Impacto:** cualquier panic mata el goroutine definitivamente.

---

## 3. 🟠 Problemas de ESCALABILIDAD

### S1. Solo 10 colores y nunca se liberan
- **Ubicación:** `pkg/handlers.go:18-30`, `pkg/handlers.go:60`
- **Detalle:**
  - Solo 10 colores definidos → límite duro de ~10 jugadores.
  - `DesconnectUser` **no devuelve el color** a `remaining` → tras 10 conectar/desconectar, se agotan. `IdentifyPlayer()` devuelve error que se ignora (`color, _ := IdentifyPlayer()`), asignando color **vacío**.

### S2. IDs de jugador sin validación
- **Ubicación:** `internal/network/ws/handlers/conn.go:12`
- **Detalle:** el `id` viene del cliente por URL (`:id`). Dos clientes con el mismo id se sobreescriben en `jg.Players[id]`. La lib `uuid` ya está como dependencia pero no se usa.

### S3. Buffer de cola sin backpressure controlado
- **Ubicación:** `internal/config/config.go:7` + `internal/network/ws/handlers/conn.go:48`
- **Detalle:** si los workers se atascan (por C1+C2), el canal de 5000 se llena; entonces `jg.Queue <- message` **bloquea la lectura de todos los clientes** (backpressure en cascada).

### S4. Arquitectura de sala única (single global `JobGame`)
- **Ubicación:** `cmd/main.go:14`
- **Detalle:** todo el servidor es UN mapa de jugadores. No hay salas/partidas. Para escalar a muchas partidas simultáneas hay que particionar.

### S5. O(N) por mensaje de movimiento + lógica dudosa
- **Ubicación:** `internal/core/players/handlers.go:35` + `internal/core/players/events.go:18`
- **Detalle:** `MovePlayer` construye el array de todos y **se lo envía al propio jugador que se movió**, no a los demás. En VR a alta frecuencia es O(N) por paquete. Parece bug funcional además de ineficiente.

---

## 4. 🟡 Problemas de PERFORMANCE / Calidad

### P1. Doble serialización JSON
- **Ubicación:** `internal/core/players/events.go:16-18` y similares
- **Detalle:** `json.Marshal(allPlayers)` → `string(bytes)` → envuelto en `MessageObject` → `json.Marshal` otra vez. Doble trabajo + allocations en el hot path.

### P2. Sin pooling de buffers
- **Detalle:** no se reutilizan buffers ni `json.Encoder` con pool. La dependencia indirecta `bytebufferpool` ya está pero no se usa. Genera presión de GC bajo carga.

### P3. `rand.Seed` deprecado
- **Ubicación:** `pkg/handlers.go:39`
- **Detalle:** obsoleto desde Go 1.20; usar `rand.New(rand.NewSource(...))`.

### P4. Sin heartbeat / ping-pong ni manejo de reconexión
- **Detalle:** conexiones zombi se acumulan (combinado con S1 = fuga de colores).

### P5. Sin graceful shutdown
- **Ubicación:** `cmd/main.go:26`
- **Detalle:** `app.Listen` sin capturar SIGTERM. Al reiniciar el contenedor se cortan todos los WS de golpe.

### P6. Logging en hot path
- **Ubicación:** varios `log.Printf` en `internal/core/players/handlers.go:13`, `pkg/handlers.go`
- **Detalle:** logs por cada jugador no encontrado / cada error de write; si muchos clientes fallan, satura I/O.

### P7. `start.sh` inválido como Dockerfile de producción
- **Ubicación:** `start.sh:13`
- **Detalle:** `CMD ["go","run","main.go"]` no compila (main en `cmd/`) y `go run` no es para producción.

### P8. JobGame sin cleanup al desconectar un cliente
- **Ubicación:** `internal/network/ws/handlers/conn.go:19-24`
- **Detalle:** el defer envía `RemovePlayer` al canal pero, si el canal está lleno/roto, el defer bloquea; además `DesconnectUser` cierra el socket y luego se encola un evento que hará write a un socket ya cerrado (error de write logueado).

---

## 5. Mapa de calor por archivo

```
internal/network/queue/worker.go  🔴🔴🔴   (lock global + writes dentro)
pkg/handlers.go                   🔴🟠🟡  (writes sin deadline, colores, rand deprecado)
internal/core/players/events.go   🔴🟡    (nil panic, doble marshal)
internal/core/players/handlers.go 🟡      (logging hot path)
internal/network/ws/handlers/conn.go 🟡    (backpressure, sin heartbeat)
internal/config/config.go         🟡      (constantes fijas)
cmd/main.go                       🟡      (sin graceful shutdown)
start.sh                          🟡      (CMD inválido)
```

---

## 6. Top 5 correcciones por impacto

| # | Corrección | Esfuerzo | Impacto |
|---|---|---|---|
| 1 | Sacar `WriteMessage` fuera del lock + `SetWriteDeadline` | M | 🔴 Crítico |
| 2 | Reemplazar `Lock()` por `RLock()` en lecturas + `sync.Map` por room | M | 🔴 Crítico |
| 3 | `recover` por worker + nil-check tras `ConvertToPlayer` | S | 🔴 Crítico |
| 4 | Pool de colores con liberación al desconectar + ampliación de paleta | S-M | 🟠 Alto |
| 5 | Generar ID server-side (UUID) + revisar lógica de MovePlayer | S | 🟠 Alto |

---

## 7. Referencias cruzadas

- Plan de implementación por fases: [`docs/ROADMAP.md`](./ROADMAP.md)
- Arquitectura actual del código: [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md)
- Log cronológico de cambios: [`docs/CHANGELOG.md`](./CHANGELOG.md)
- Contrato de red (congelado): [`../SPEC.md`](../SPEC.md)

---

## 8. Estado de resolución

Mapeo de cada issue identificado a la fase que lo resolvió (o nota si sigue pendiente).

### 🔴 Críticos (C1-C5)

| # | Issue | Fase | Archivo de fix | Notas |
|---|---|---|---|---|
| C1 | Lock global serializa todo el servidor | ✅ A1.2 + F1.2 | `internal/network/queue/worker.go` | `RLock` en lecturas, `Lock` solo en `MovePlayer` (mutación inevitable) |
| C2 | Escrituras de red DENTRO del lock | ✅ A1.1 | `pkg/handlers.go` (`StartWriter`) | Workers solo encolan al outbox; escritura la hace goroutine dedicada |
| C3 | Sin WriteDeadline en los sockets | ✅ A1.1 | `pkg/handlers.go` (`StartWriter`) | `SetWriteDeadline(2s)` antes de cada `WriteMessage` |
| C4 | Panic potencial por nil pointer | ✅ A1.3 + F1.2 | `internal/core/players/events.go` | `ConvertToPlayer` retorna nil → `EventMovement` detecta y descarta |
| C5 | Sin recovery en workers | ✅ A1.3 | `internal/network/queue/worker.go` | `defer recover` por mensaje + por goroutine |

### 🟠 Escalabilidad (S1-S5)

| # | Issue | Fase | Archivo de fix | Notas |
|---|---|---|---|---|
| S1 | Solo 10 colores y nunca se liberan | ✅ A2.1 + F2.1 + F2-fix.2 | `internal/colors/pool.go` | Pool HSL infinito (~4320 únicos) con `Acquire`/`Release` |
| S2 | IDs de jugador sin validación | ✅ A2.2 | `pkg/handlers.go` (`AddUser`) | Rechazo si id duplicado |
| S3 | Buffer de cola sin backpressure controlado | 🟡 Parcial | — | Outbox per-conn tiene drop-on-full; cola global aún bloquea productor |
| S4 | Arquitectura de sala única | ⬜ Pendiente | — | Fuera de scope (requiere refactor mayor) |
| S5 | O(N) por mensaje + lógica dudosa | ⬜ Preservado | — | Comportamiento emisor-only es explícito en SPEC §5.2 |

### 🟡 Performance / Calidad (P1-P8)

| # | Issue | Fase | Archivo de fix | Notas |
|---|---|---|---|---|
| P1 | Doble serialización JSON | ⬜ Pendiente | — | Optimizable con `bytebufferpool` o `json.Encoder` con pool |
| P2 | Sin pooling de buffers | ⬜ Pendiente | — | Dependencia indirecta existe, no se usa |
| P3 | `rand.Seed` deprecado | ✅ A2.1 | `internal/colors/pool.go` | Pool HSL no usa rand |
| P4 | Sin heartbeat / ping-pong | ✅ A2.3 + F2.3 + F2-fix.1 | `internal/network/ws/handlers/conn.go` | Keepalive pasivo: `SetReadDeadline` + `SetPongHandler`. **No pinger activo** (race con fasthttp) |
| P5 | Sin graceful shutdown | ✅ A2.3 | `cmd/main.go` | `signal.NotifyContext` + `app.ShutdownWithContext(10s)` |
| P6 | Logging en hot path | ✅ A2.3 | `pkg/handlers.go`, `cmd/main.go` | zerolog con niveles; logs costosos en `Debug` solo |
| P7 | `start.sh` inválido como Dockerfile de producción | ⬜ Pendiente | — | `start.sh` y `Dockerfile` sin tocar |
| P8 | JobGame sin cleanup al desconectar | ✅ A1.1 + A2.3 | `pkg/handlers.go` (`DesconnectUser`) | `Release` color, `close(Stop)`, `close(Outbox)` |

### Resumen

| Categoría | Total | ✅ Resueltos | 🟡 Parcial | ⬜ Pendientes |
|---|---|---|---|---|
| 🔴 Críticos (C1-C5) | 5 | **5** | 0 | 0 |
| 🟠 Escalabilidad (S1-S5) | 5 | **2** | 1 | 2 |
| 🟡 Performance (P1-P8) | 8 | **4** | 0 | 4 |
| **Total** | **18** | **11** | **1** | **6** |

> **Calificación estimada post-refactor**: 3/10 → **6.5/10** (issues críticos eliminados; quedan mejoras de performance y arquitectura multi-sala para fases futuras).
