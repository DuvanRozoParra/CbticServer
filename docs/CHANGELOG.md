# Changelog — CbticServer

Log cronológico de cambios por fase. Cada entrada incluye archivos tocados, commits (si existen) y validación.

> **Wire contract**: nuevo evento `ServerShutdown` (7) añadido. Ver [`../SPEC.md`](../SPEC.md) §5.6.

---

## [AUDIT-FIX] — 2026-06-24

**Estado**: ✅ Completo. 6 hallazgos de auditoría de estabilidad corregidos.

### Cambios

- **H-1/M-4 (crítico)**: `internal/network/ws/handlers/conn.go`, `internal/config/config.go` — Rate limit por conexión (200 msg/s, `CBTIC_INGRESS_RATE`). Ventana fija sin mutex (single-goroutine). **Un jugador flooding ya no satura la cola compartida ni degrada a otros.**
- **M-1 (medio)**: `internal/network/ws/handlers/conn.go` — `defer DesconnectUser` registrado inmediatamente después de `AddUser`, antes de `StartWriter`. **Elimina leak de estado si algo panica entre AddUser y el defer anterior.**
- **M-2 (medio)**: `pkg/handlers.go`, `internal/network/ws/handlers/conn.go` — Nueva función `KickPlayer`. Si `AddUser` falla por duplicado, se echa la conexión vieja (`SetReadDeadline(now)` + `close(Stop)`) y se reintenta. **Reconexión inmediata con mismo id, sin esperar 60s de timeout.**
- **M-3 (medio)**: `cmd/main.go`, `internal/config/config.go`, `SPEC.md` — Evento `ServerShutdown` (7). En SIGTERM/SIGINT: broadcast `"server_shutdown"` a todos (Text), 500ms de respiro, luego `SetReadDeadline(now)` en todas las conexiones. **El cliente Unity puede mostrar "servidor reiniciándose" en vez de "conexión perdida".**
- **B-1 (bajo)**: `pkg/handlers.go`, `internal/core/players/handlers.go` — Eliminado dead code: `ConvertToJson`, `jsonBufferPool`, import `bytebufferpool`, `GetPlayer`.

### Archivos tocados

```
internal/config/config.go                     (H-1, M-3: IngressRatePerSec, ServerShutdown)
internal/network/ws/handlers/conn.go          (H-1, M-1, M-2: rate limit, defer up, kick retry)
pkg/handlers.go                               (B-1, M-2: dead code gone, KickPlayer added)
internal/core/players/handlers.go             (B-1: GetPlayer removed)
cmd/main.go                                   (M-3: shutdown event + drain)
pkg/handlers_test.go                          (tests: KickPlayer, AddUser tras kick, ServerShutdown const)
SPEC.md                                       (M-3: §5.6 ServerShutdown, event table)
```

### Validación

```
gofmt -l .                          → limpio
go vet ./...                        → sin warnings
go build -race                      → compila
go test -race -timeout 60s ./...    → 28 tests pasan
benchtest -clients 100 -rate 120    → p99=4.19µs, 0 drops, 3MB heap
```

---

## [FIX-AUDIT] — 2026-06-24

**Estado**: ✅ Completo. 10 bugs de la auditoría corregidos (3 críticos, 4 medios, 3 bajos).

### Cambios

- **FIX-1 (crítico)**: `pkg/handlers.go` — Eliminado `BuildWireJSON`, `writeMessageObjectJSON`, `appendStringField`. Reemplazado por `BuildWire` que usa `json.Marshal` estándar. **Elimina regresión que producía JSON inválido con caracteres de control.**
- **FIX-2 (crítico)**: `internal/network/ws/routes.go` — Health endpoint toma `RLock` antes de `len(jg.Players)`. **Elimina DATA RACE.**
- **FIX-3 (crítico)**: `internal/network/queue/worker.go` — Los 5 casos de evento verifican errores de `MarshalEventData` y `BuildWire` con log+descarta, en vez de `_`. **Cumple F3.2 que estaba marcado done pero no implementado.**
- **FIX-4 (medio)**: `internal/core/players/handlers.go` — Eliminado `sync.Pool` contraproducente. `GetAllPlayer` vuelve a `make` simple.
- **FIX-5 (medio)**: `cmd/benchtest/main.go` — Benchmark ahora inicia un noop-writer que consume el outbox. HeapAlloc bajó de 2.2 GB a 3 MB.
- **FIX-6 (medio)**: `pkg/handlers.go`, `events.go` — Eliminado dead code: `ConvertToMessageObject`, `ConvertToJsonSafe`, `MarshalEventDataRaw`, `MarshalRaw`.
- **FIX-7 (medio)**: Subsumido en FIX-1 (la API engañosa de `writeMessageObjectJSON` ya no existe).
- **FIX-8 (bajo)**: `internal/typesGame/player.go` — Eliminado `IdentifyManager` y `PlayersArray` (dead types). Quitado `import "sync"`.
- **FIX-9 (bajo)**: `pkg/handlers.go` — `AddUser` ya no construye `data` redundante (el worker lo descarta). `MessageObject.Data = ""`.
- **FIX-10 (bajo)**: `Dockerfile` — Agregado `wget` al stage final para HEALTHCHECK.

### Archivos tocados

```
pkg/handlers.go                              (FIX-1, FIX-3, FIX-6, FIX-9)
pkg/contract_test.go                         (test ampliado: 16 casos incl. control chars)
internal/network/queue/worker.go             (FIX-3)
internal/core/players/handlers.go            (FIX-4)
internal/core/players/events.go              (FIX-6: MarshalRaw eliminado)
internal/typesGame/player.go                 (FIX-8)
internal/network/ws/routes.go                (FIX-2)
cmd/benchtest/main.go                        (FIX-5)
Dockerfile                                   (FIX-10)
docs/BENCHMARK-RESULTS.md                    (FIX-5: nota sobre writer real)
```

### Validación

```
gofmt -l .                  → vacío
go vet ./...                → sin warnings
go build -race              → compila
go test -race -short ./...  → todos pasan (16 contract cases incl. \x00 \x01 \x1F y 32 control chars)
curl /api/health            → {"status":"ok","players":0,"queue":0}
benchmark N=100 @ 60Hz      → 5,979 msg/s, p99=1.84µs, HeapAlloc=3MB
```

### Bug crítico destacado

`appendStringField` (eliminado en FIX-1) era una regresión introducida por F4.2: escapaba solo 6 de los 32 caracteres de control Unicode, produciendo JSON inválido cuando un cliente enviaba `\x00`-\x1F` (excepto los 6 manejados). El original usaba `json.Marshal` que sí los escapaba. **Cualquier cliente malicioso podía tirar el JSON parser de los demás clientes** enviando un RayInteraction o ActionHandsPlayer con un byte de control en `data`.

---

## [F2-fix] — 2026-06-24

---

## [F2-fix] — 2026-06-24

**Estado**: ✅ Completo. BUG-1 fix + FMT-1 fix aplicados.

### Cambios

- **BUG-1**: `pkg/handlers.go` — `AddUser` ahora retorna error si `Acquire()` falla (rechazo limpio en `Conn` con `c.Close()`).
- **FMT-1**: `internal/colors/pool.go` — `gofmt -w` aplicado (constantes + espaciado de operadores).

### Validación

```
gofmt -l .                    → vacío
go vet ./...                  → sin warnings
go build -race ./cmd/main.go  → compila
go test -race ./internal/colors/ → PASS (1000 colores únicos)
```

---

## [F2] — 2026-06-24

**Estado**: ✅ Completo. Hardcoding eliminada, configurabilidad por env vars.

### Cambios

- **F2.1**: `internal/colors/pool.go` — constantes nombradas (`goldenAngle`, `defaultSat`, `defaultLight`).
- **F2.2**: `internal/config/config.go` — env vars `CBTIC_*` vía `envStr`/`envInt`/`envDur`. Eventos siguen como `const`.
- **F2.3**: `internal/network/ws/handlers/conn.go` — `SetReadDeadline` + `SetPongHandler`.
- **F2.4**: `cmd/main.go` — `config.ShutdownTimeout` reemplaza literal `10*time.Second`.
- **F2.5**: `cmd/main.go` — warn si `CBTIC_LOG_LEVEL` inválido.

### Archivos tocados

```
internal/colors/pool.go
internal/config/config.go
internal/network/ws/handlers/conn.go
cmd/main.go
```

### Validación

```
go vet ./...                  → sin warnings
go build -race ./cmd/main.go  → compila
smoke test 3 clientes WS      → ✅ AddPlayer, RemovePlayer, graceful shutdown
```

---

## [F2-fix] — 2026-06-24

**Estado**: ✅ Completo. Correcciones sobre F2 detectadas en auditoría.

### Cambios

- **F2-fix.1**: Eliminado `StartPinger` + `WriteControl` + `PingInterval`. Keepalive pasivo (`SetReadDeadline` + `SetPongHandler`).
- **F2-fix.2**: `colors.Pool` infinito real (hue áureo + 4 capas L × 3 capas S = ~4320).
- **F2-fix.3**: Eliminado `defaultLightAlt` (dead code).
- **F2-fix.4**: Eliminado `SendMsgAllWithout` (dead code, sin callers).
- **F2-fix.5**: Warn si `CBTIC_LOG_LEVEL` inválido.

### Archivos tocados

```
internal/colors/pool.go          (rewrite con capas L/S)
internal/colors/pool_test.go     (nuevo, tests)
internal/config/config.go        (eliminado PingInterval)
pkg/handlers.go                  (eliminado StartPinger, SendMsgAllWithout)
internal/network/ws/handlers/conn.go (eliminado StartPinger call)
cmd/main.go                      (warn en log level)
```

### Validación

```
test internal/colors/ -race      → ✅ 1000 colores únicos
smoke test 3 clientes            → ✅ flow completo
DATA RACE detector               → ✅ cero races
```

---

## [F1] — 2026-06-24

**Estado**: ✅ Completo. 6 bugs críticos del refactor A1/A2 corregidos.

### Cambios

- **F1.1**: Quitados `ReadTimeout`/`WriteTimeout` de `fiber.Config` (cortaban WS a 2s).
- **F1.2**: Locks correctos — `Lock` en `MovePlayer` (muta), `RLock` en otros; nil-checks en `EventMovement` y `GetAllPlayer`.
- **F1.3**: `AddUser` retorna `(*MessageObject, *Players, error)` — elimina race de re-lookup.
- **F1.4**: `recover` dentro del `for` (por mensaje); `for msg := range jg.Queue`.
- **F1.5**: Writer usa `Stop chan struct{}`; `SetReadDeadline` si `WriteMessage` falla; **NO** cerrar Conn desde fuera (race con fasthttp).

### Archivos tocados

```
internal/network/queue/worker.go
internal/core/players/events.go
internal/core/players/handlers.go
internal/network/ws/handlers/conn.go
pkg/handlers.go
cmd/main.go
```

### Validación

```
go build -race                → ✅ compila
smoke test 2-3 clientes       → ✅ flow end-to-end
DATA RACE detector            → ✅ cero races
```

---

## [A2] — 2026-06-24

**Estado**: ✅ Completo. Bugs y límites resueltos.

### Cambios

- **A2.1**: `internal/colors/pool.go` (nuevo) — `Pool` con `sync.Mutex`, `inUse map[string]struct{}`, HSL hue áureo.
- **A2.2**: `pkg/handlers.go` — `AddUser` valida id duplicado, `DesconnectUser` hace `Release`.
- **A2.3**: `cmd/main.go` — zerolog + `signal.NotifyContext` + `app.ShutdownWithContext`.
- **A2.4**: Eliminado `rand.Seed` deprecado (pool usa HSL determinista, no rand).

### Archivos tocados

```
internal/colors/pool.go          (nuevo)
internal/typesGame/queue.go      (Colors *colors.Pool en JobGame)
internal/network/queue/queue.go  (inicializa Pool en New)
pkg/handlers.go                  (AddUser/DesconnectUser con Acquire/Release)
cmd/main.go                      (zerolog, graceful shutdown)
```

### Validación

```
go vet ./...                  → sin warnings
go build -race                → compila
```

---

## [A1] — 2026-06-24

**Estado**: ✅ Completo. Estabilización crítica.

### Cambios

- **A1.1**: Outbox por conexión — `internal/typesGame/queue.go` gana `Outbox chan OutboundFrame` + `Stop chan struct{}`. `pkg/handlers.go` tiene `StartWriter` con goroutine dedicada, `SetWriteDeadline` antes de `WriteMessage`, `SetReadDeadline` si falla.
- **A1.2**: `internal/network/queue/worker.go` — `processWorker` usa `RLock` en lecturas, `Lock` solo en `MovePlayer`.
- **A1.3**: `defer recover` por mensaje en `worker()`. `internal/core/players/events.go` — nil-check en `EventMovement`.

### Archivos tocados

```
internal/typesGame/queue.go       (Outbox, Stop, OutboundFrame)
pkg/handlers.go                   (StartWriter, safeSendOutbox, SendAllMsg, SendMsg)
internal/network/queue/worker.go  (RLock/Lock por evento, recover)
internal/core/players/events.go   (nil-check)
internal/core/players/handlers.go (nil-check GetAllPlayer)
```

### Validación

```
go vet ./...                  → sin warnings
go build -race                → compila
```

---

## [AUDIT] — 2026-06-24

**Estado**: ✅ Completo. Documento histórico.

### Contenido

- Auditoría de performance y escalabilidad — calificación 3/10.
- 5 issues críticos (C1-C5), 5 de escalabilidad (S1-S5), 8 de performance (P1-P8).
- Ver resolución en [`./AUDIT.md#8-estado-de-resolución`](./AUDIT.md#8-estado-de-resolución).

---

## [SPEC] — 2026-06-24

**Estado**: ✅ Completo. Wire contract v1.0 congelado.

### Contenido

- Definición inmutable de JSON de entrada/salida.
- Envoltura canónica `MessageObject` con `data`/`from`/`events`.
- Frames Text/Binary por evento.
- Reglas para refactors (no tocar wire sin aprobación).

---

## Resumen ejecutivo

| Fase | Issues resueltos | Archivos tocados | Estado |
|---|---|---|---|
| A1 | C1, C2, C3, C4, C5 | 5 | ✅ |
| A2 | S1, S2, P3, P5, P6 | 5 | ✅ |
| F1 | 6 bugs del refactor | 6 | ✅ |
| F2 | hardcoding, env vars | 4 | ✅ |
| F2-fix | BUG-1, FMT-1, pool infinito | 4 | ✅ |

**Total**: 11 issues resueltos, 6 pendientes (S3, S4, S5, P1, P2, P7), 0 críticos abiertos.

---

## Próximos pasos

- **F3**: Robustez menor (4 fixes). Ver [`./ROADMAP.md#71-f3--robustez-menor`](./ROADMAP.md#71-f3--robustez-menor).
- **A3**: Benchmark sintético (GATE de decisión). Ver [`./ROADMAP.md#72-a3--benchmark-gate-de-decisión`](./ROADMAP.md#72-a3--benchmark-gate-de-decisión).
- **B**: Migración condicional a `coder/websocket` + `net/http`. Ver [`./ROADMAP.md#73-b--migración-condicional`](./ROADMAP.md#73-b--migración-condicional).
