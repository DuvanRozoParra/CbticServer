# Roadmap — Plan de Implementación Consolidado

**Objetivo:** llevar el servidor de **3/10 → 8+/10** en performance y escalabilidad sin romper el contrato de red definido en [`../SPEC.md`](../SPEC.md).

**Regla de oro:** los JSON de entrada/salida y los tipos de frame por evento están **congelados** (SPEC §5). Toda modificación interna requiere aprobación explícita antes de codificar.

**Estrategia aprobada:** **Camino A → Gate (benchmark) → Camino B condicional**. Refactor in-place primero, decidir migración a `coder/websocket` + `net/http` solo si los datos lo justifican.

---

## 1. Estado actual (snapshot)

| Campo | Valor |
|---|---|
| Versión del servidor | post-F2-fix (refactor in-place) |
| Calificación estimada | **6.5 / 10** |
| Issues críticos (C1-C5) | ✅ todos resueltos |
| Issues escalabilidad (S1-S5) | 🟡 S1, S2, S3 resueltos; S4, S5 sin tocar |
| Issues performance (P1-P8) | 🟡 P3, P5, P6 resueltos; resto sin tocar |
| Wire contract | ✅ **intacto** (golden bytes verificados) |
| Data race detector | ✅ **cero races** en `go build -race` |

---

## 2. Decisiones fijadas

| Punto | Decisión | Notas |
|---|---|---|
| Camino | **A → gate → B** | Refactor primero, migración con datos |
| Librería WS objetivo (si se migra) | **coder/websocket** | API moderna, writes concurrent-safe |
| HTTP framework (si se migra) | **net/http estándar** | Salir de Fiber |
| ID de jugador | **`:id` por URL** | SPEC §1 intacto, sin bump |
| Logger | **zerolog** | JSON structured, perf alta |
| Paleta de colores | **Pool HSL infinito** | ~4320 únicos, formato `#RRGGBB` |
| Destino `MovePlayer` | **Emisor-only** | Preservar comportamiento actual |
| Keepalive | **Pasivo** (`SetReadDeadline` + `SetPongHandler`) | No pinger activo (race con fasthttp) |
| Tests de contrato | **Golden bytes** | Bytes idénticos antes/después por evento |

> ✅ Con esto **NO se rompe ningún contrato del SPEC §5**. La migración a coder/websocket tampoco lo rompe (frames Text/Binary por evento se respetan).

---

## 3. Lo que NO cambia (SPEC intacto)

- Endpoint `GET /api/v1/ws/:id`, puerto `8080`, sin subprotocolos.
- Envoltura `MessageObject` con `data`/`from`/`events` (plural).
- `data` siempre string (con doble capa JSON donde aplique).
- Frames Text/Binary por evento (SPEC §2).
- Destinatarios por evento (incluido `MovePlayer` al emisor).
- Transformación `body.position.y -= 0.3` en `MovePlayer`.
- Paleta en formato `#RRGGBB` (independiente del método de generación).
- `AddPlayerMsg` con `id` + `color`.

---

## 4. Progreso por fase

| # | Fase | Estado | Entregable |
|---|---|---|---|
| 1 | **A1** Estabilización | ✅ | Outbox + writer goroutine + WriteDeadline |
| 2 | **A1** Quitar lock global del hot path | ✅ | `RLock` en lecturas, `Lock` en mutación |
| 3 | **A1** `recover` por worker + nil-check | ✅ | `defer recover` por mensaje |
| 4 | **A2** Pool de colores HSL | ✅ | `colors.Pool` con `Acquire`/`Release` |
| 5 | **A2** Validación `:id` único | ✅ | Rechazo si id duplicado |
| 6 | **A2** zerolog + graceful shutdown | ✅ | Logger estructurado + SIGTERM |
| 7 | **A2** Quitar `rand.Seed` deprecado | ✅ | Pool usa HSL, sin rand |
| 8 | **F1** Fixes críticos de A1/A2 | ✅ | 6 bugs (deadline, locks, race, recover) |
| 9 | **F2** Hardcoding/configurabilidad | ✅ | Env vars, constantes nombradas, pool HSL |
| 10 | **F2-fix** Pool infinito + cleanup | ✅ | Capas L/S, sin pinger, sin dead code |
| 11 | **A3** Benchmark sintético (GATE) | ⬜ | Carga 60Hz con N=10/50/100/200 |
| 12 | **F3** Robustez menor | ⬜ | Errors propagados, nil guards, logs |
| 13 | **B** Migración (condicional) | ⬜ | Solo si A3 lo justifica |

---

## 5. Diagrama de fases (actualizado)

```
┌──────────────────────────────────────────────────────────────┐
│ FASE A1: Estabilización ✅                                   │
│  • Outbox por conexión (writer goroutine + WriteDeadline)    │
│  • Quitar lock global del hot path (RLock en lecturas)       │
│  • recover por worker + nil-check en ConvertToPlayer        │
└──────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────┐
│ FASE A2: Bugs y límites ✅                                   │
│  • Pool de colores HSL con Release al desconectar            │
│  • Validación de :id único                                   │
│  • zerolog + graceful shutdown                               │
└──────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────┐
│ FASE F1: Fixes críticos del refactor A1/A2 ✅                │
│  • Quitar ReadTimeout/WriteTimeout de fiber.Config           │
│  • Locks correctos (Lock en MovePlayer, RLock en otros)      │
│  • AddUser retorna (*MessageObject, *Players, error)         │
│  • recover por mensaje + writer con Stop channel             │
└──────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────┐
│ FASE F2: Hardcoding/configurabilidad ✅                      │
│  • Pool HSL con capas L/S (infinito real)                    │
│  • Env vars CBTIC_* (config)                                 │
│  • Keepalive pasivo (sin pinger activo)                      │
│  • AddUser rechaza si Acquire falla                          │
└──────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────┐
│ FASE A3: Benchmark (GATE) ⬜ PENDIENTE                        │
│  • Test sintético: N=10, 50, 100, 200 a 60 Hz               │
│  • Métricas: msg/s, p99, CPU, allocs/op, goroutines          │
│  • Criterio: p99<50ms@100cli + CPU<70% → NO migrar           │
└──────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
     ┌──────────────────┐            ┌──────────────────────┐
     │ Gate: APROBADO   │            │ Gate: FALLA          │
     │ No migrar        │            │ Ejecutar Camino B    │
     └──────────────────┘            └──────────────────────┘
                                               │
                                               ▼
                               ┌────────────────────────────────┐
                               │ FASE B: Migración condicional  │
                               │  • B1 Interfaz Port             │
                               │  • B2 coder/websocket + net/http│
                               │  • B3 Benchmark A/B             │
                               │  • B4 Rollout con flag          │
                               └────────────────────────────────┘
```

---

## 6. Detalle de fases completadas

### A1 — Estabilización ✅
- **`internal/typesGame/queue.go`**: `Players` gana `Outbox chan OutboundFrame` + `Stop chan struct{}`.
- **`pkg/handlers.go`**: `StartWriter` corre goroutine dedicada, lee del outbox, `SetWriteDeadline` antes de `WriteMessage`, `SetReadDeadline` si falla.
- **`internal/network/queue/worker.go`**: `processWorker` usa `RLock` en lecturas, `Lock` solo en `MovePlayer` (mutación).

### A2 — Bugs y límites ✅
- **`internal/colors/pool.go`**: `Pool` con `sync.Mutex`, `inUse map[string]struct{}`, HSL hue áureo.
- **`pkg/handlers.go`**: `AddUser` valida id duplicado + `Acquire` color, `DesconnectUser` hace `Release`.
- **`cmd/main.go`**: zerolog + `signal.NotifyContext` + `app.ShutdownWithContext`.

### F1 — Fixes críticos ✅
- F1.1: Quitados `ReadTimeout`/`WriteTimeout` de `fiber.Config` (cortaban WS a 2s).
- F1.2: Locks correctos — `Lock` en `MovePlayer`, `RLock` en otros; nil-checks.
- F1.3: `AddUser` retorna `(*MessageObject, *Players, error)` — elimina race de re-lookup.
- F1.4: `recover` dentro del `for` (por mensaje).
- F1.5: Writer usa `Stop` channel; `SetReadDeadline` si `WriteMessage` falla; **NO** cerrar `Conn` desde fuera (race con fasthttp).

### F2 + F2-fix — Hardcoding/configurabilidad + pool infinito ✅
- **F2.1**: `colors.Pool` con `goldenAngle=137.5077640500378`, constantes nombradas.
- **F2.2**: `config.go` con env vars (`CBTIC_*`) vía helpers `envStr`/`envInt`/`envDur`.
- **F2.3**: `SetReadDeadline` + `SetPongHandler` (keepalive pasivo).
- **F2.4**: `config.ShutdownTimeout` reemplaza literal `10*time.Second`.
- **F2-fix.1**: Eliminado `StartPinger` (race con fasthttp `releaseConn`).
- **F2-fix.2**: `colors.Pool` infinito real (hue áureo + 4 capas L × 3 capas S = ~4320).
- **F2-fix.3**: Eliminado `defaultLightAlt` (dead code).
- **F2-fix.4**: Eliminado `SendMsgAllWithout` (dead code, sin callers).
- **F2-fix.5**: Warn si `CBTIC_LOG_LEVEL` inválido.
- **BUG-1 fix**: `AddUser` retorna error si `Acquire()` falla (rechazo limpio en `Conn`).
- **FMT-1 fix**: `gofmt` aplicado a `internal/colors/pool.go`.

---

## 7. Fases pendientes

### 7.1 F3 — Robustez menor ⬜

| Fix | Archivo | Descripción |
|---|---|---|
| F3.1 | `internal/core/players/events.go` | `ConvertToJson` propaga error en lugar de loguear + retornar nil |
| F3.2 | `pkg/handlers.go` | Validar `jg.Colors != nil` en `AddUser` (consistencia con `DesconnectUser`) |
| F3.3 | `internal/network/ws/handlers/conn.go` | Loguear si `c.Close()` falla tras `AddUser` error |
| F3.4 | `internal/network/ws/handlers/conn.go` | `SetReadLimit` para prevenir OOM con mensajes gigantes |

**Criterio de éxito F3**: servidor rechaza payloads inválidos sin entrar a estado inconsistente; un cliente malicioso no puede tirar el servidor con un mensaje de 1GB.

### 7.2 A3 — Benchmark (GATE de decisión) ⬜

**Objetivo:** decidir si migrar a `coder/websocket` + `net/http` o quedarse en Fiber.

**Setup:**
- N conexiones cliente simuladas (N = 10, 50, 100, 200).
- Cada cliente envía `MovePlayer` a 60 Hz durante 60 s.

**Métricas:**
- `msg/s` agregados (entrada + salida).
- Latencia `p50` / `p95` / `p99` por mensaje.
- CPU total del proceso.
- Allocations/op (vía `runtime.MemStats` o `pprof`).
- Goroutines activas.
- Memoria por conexión (`heap alloc / N`).

**Criterio de aprobación del gate:**

| Resultado a N=100 | Decisión |
|---|---|
| p99 < 50 ms **y** CPU < 70 % | ✅ **NO migrar** (cerrar Camino B) |
| p99 ≥ 50 ms **o** CPU ≥ 70 % y perfil apunta a la stack WS | ⚠️ Ejecutar **Fase B** |
| p99 ≥ 50 ms **o** CPU ≥ 70 % pero perfil NO apunta a WS | ❌ Optimizar hot path de la app (volver a A1/A2) |

**Entregable**: informe de benchmark publicado en `docs/BENCHMARK-RESULTS.md` + decisión binaria documentada.

### 7.3 B — Migración (CONDICIONAL) ⬜

Solo si A3 lo justifica. Secuencia segura con `coder/websocket` + `net/http`.

#### B1 — Interfaz de puerto (desacoplar)
- **Archivo**: nuevo `internal/network/ws/port/port.go`
- **Contenido**:
  ```go
  type Conn interface {
      ReadMessage() (int, []byte, error)
      WriteMessage(opcode int, p []byte) error
      SetWriteDeadline(t time.Time) error
      Close() error
  }
  type Upgrader interface {
      Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (Conn, error)
  }
  ```

#### B2 — Implementación `coder/websocket` + `net/http`
- `net/http.ServeMux` con ruta `/api/v1/ws/{id}` (compatible con cliente).
- `websocket.Accept(w, r, nil)` (sin subprotocolos, respeta SPEC §1).
- Reutilizar el `outbox` y `writer` goroutine de A1.
- **Tests de contrato**: comparar bytes de cada evento (SPEC §5) contra fixtures golden — deben ser idénticos.

#### B3 — Benchmark A/B lado a lado
- Mismo harness de A3, contra ambas implementaciones.
- **Criterio**: ≥ 25 % mejora en `msg/s` **o** `p99` para justificar el cambio.

#### B4 — Rollout controlado
- Feature flag `WS_BACKEND=fiber|coder` (variable de entorno).
- Rollback inmediato ante regresión.
- Pruebas de integración con el cliente Unity real.

---

## 8. Resumen de archivos tocados

```
internal/colors/pool.go           ← F2.1, F2-fix.2 (Pool HSL infinito)
internal/colors/pool_test.go      ← F2-fix.2 (tests)
internal/config/config.go         ← F2.2 (env vars)
internal/core/players/events.go   ← A1.3 (nil-check)
internal/core/players/handlers.go ← A1.3 (nil-check GetAllPlayer)
internal/network/queue/queue.go   ← A2 (Colors en JobGame)
internal/network/queue/worker.go  ← A1.2, A1.3, F1.2 (locks, recover)
internal/network/ws/handlers/conn.go ← A1, A2.2, F1.5, F2.3 (writer, deadlines, no pinger)
internal/network/ws/routes.go     ← (sin cambios, solo verificación)
internal/network/ws/middleware/websocket.go ← (sin cambios)
internal/typesGame/queue.go       ← A1.1, A2 (Outbox, Stop channel)
pkg/handlers.go                   ← A1, A2, F1, F2, F2-fix, BUG-1 fix
cmd/main.go                       ← A2.3, F2.5 (zerolog, graceful shutdown, warn)
```

---

## 9. Preguntas abiertas resueltas

| # | Pregunta | Resolución |
|---|---|---|
| 1 | Camino | **A → gate → B** ✅ |
| 2 | Librería WS objetivo | **coder/websocket** ✅ |
| 3 | HTTP framework | **net/http estándar** ✅ |
| 4 | ID jugador | **`:id` por URL** (intacto) ✅ |
| 5 | Logger | **zerolog** ✅ |
| 6 | Paleta colores | **Pool HSL infinito** ✅ |
| 7 | `MovePlayer` destino | **Emisor-only** (preservado) ✅ |
| 8 | Keepalive | **Pasivo** (sin pinger activo) ✅ |
| 9 | Tests de contrato | **Golden bytes por evento** ✅ |

---

## 10. Referencias

- Contrato de red (congelado): [`../SPEC.md`](../SPEC.md)
- Auditoría inicial (3/10): [`./AUDIT.md`](./AUDIT.md)
- Arquitectura actual: [`./ARCHITECTURE.md`](./ARCHITECTURE.md)
- Log de cambios: [`./CHANGELOG.md`](./CHANGELOG.md)
- Resultados del benchmark (cuando exista): `./BENCHMARK-RESULTS.md`
