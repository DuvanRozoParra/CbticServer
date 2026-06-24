# Arquitectura Actual — CbticServer (post-F2-fix)

Snapshot del código tras refactor in-place. Refleja el estado real de los archivos en `cmd/`, `internal/` y `pkg/`.

> **Diferencia vs. versión original** (`commit 481a9b3`, 3/10): outbox por conexión, writer goroutine con `Stop` channel, `RLock`/`Lock` por evento, `colors.Pool` HSL infinito, `recover` por mensaje, keepalive pasivo, config por env vars.

---

## 1. Vista general

```
                    ┌──────────────────────┐
                    │  Cliente Unity (×N)  │
                    │  endel/NativeWebSocket│
                    └─────────┬────────────┘
                              │  WS (RFC 6455)
                              │  /api/v1/ws/:id
                              ▼
              ┌────────────────────────────────┐
              │  Fiber app (Go)                │
              │  ─ middleware ConnectWebsocket │
              │  ─ ruta /api/v1/ws/:id         │
              │  ─ DisableStartupMessage       │
              └─────────┬──────────────────────┘
                        │
                        ▼
              ┌────────────────────────────────┐
              │  handlers.Conn (goroutine × N)│
              │  ─ AddUser (lock, valida)     │
              │  ─ ReadMessage (deadline)      │
              │  ─ encola → jg.Queue          │
              │  ─ defer: DesconnectUser      │
              └─────────┬──────────────────────┘
                        │  chan MessageObject (buf 5000)
                        ▼
              ┌────────────────────────────────┐
              │  Workers (6 goroutines)        │
              │  ── jg.Mu.RLock() lecturas    │
              │  ── jg.Mu.Lock()  MovePlayer  │
              │  ── recover por mensaje        │
              │  ── Solo encola a outbox       │
              └─────────┬──────────────────────┘
                        │  Outbox (per-conn, buf 256)
                        ▼
              ┌────────────────────────────────┐
              │  Writer goroutine (per-conn)   │
              │  ── SetWriteDeadline            │
              │  ── WriteMessage                │
              │  ── Stop channel (cierre limpio)│
              └─────────┬──────────────────────┘
                        │
                        ▼
              ┌────────────────────────────────┐
              │  WebSocket conns               │
              │  (*websocket.Conn — fasthttp)  │
              └────────────────────────────────┘
```

**Diferencia clave**: los workers **nunca tocan el socket**. Solo encolan frames a un outbox por conexión. La escritura la hace una goroutine dedicada por socket, fuera del lock, con deadline.

---

## 2. Árbol de paquetes (estado actual)

```
github.com/DuvanRozoParra/servercbtic
├── cmd/
│   └── main.go                    # entrypoint: config, queue, workers, fiber, shutdown
├── internal/
│   ├── colors/
│   │   ├── pool.go                # Pool HSL infinito (Acquire/Release)
│   │   └── pool_test.go           # tests
│   ├── config/
│   │   └── config.go              # env vars CBTIC_* + const eventos
│   ├── core/
│   │   └── players/
│   │       ├── events.go          # AddPlayer / MovePlayer / Ray / Hands / Remove
│   │       ├── handlers.go        # GetPlayer, GetAllPlayer, ConvertToPlayer
│   │       └── helpers.go         # (vacío)
│   ├── network/
│   │   ├── queue/
│   │   │   ├── queue.go           # New() → *JobGame (con Colors)
│   │   │   └── worker.go          # StartWorkers, worker, processWorker
│   │   └── ws/
│   │       ├── routes.go          # RouterWebsocket
│   │       ├── middleware/
│   │       │   └── websocket.go   # ConnectWebsocket (valida upgrade)
│   │       └── handlers/
│   │           └── conn.go        # Conn() — AddUser, read loop, writer
│   └── typesGame/
│       ├── player.go              # Player, BodyPart, Vector3, Quaternion, AddPlayerMsg
│       ├── conn.go                # MessageObject (envoltura canónica)
│       └── queue.go               # JobGame, Players, OutboundFrame
└── pkg/
    └── handlers.go                # AddUser, DesconnectUser, SendAllMsg, SendMsg, safeSendOutbox, StartWriter
```

---

## 3. Flujo detallado por evento

### 3.1 Conexión y `AddPlayer`

1. Cliente hace upgrade a `GET /api/v1/ws/:id`.
2. Middleware `ConnectWebsocket` valida el upgrade; si no es WS → 426.
3. Fiber instancia `websocket.Conn` y llama a `handlers.Conn(c, jg)`.
4. `Conn` (`internal/network/ws/handlers/conn.go`):
   - Extrae `id` de la URL.
   - `pkg.AddUser(jg, c, id)`:
     - Toma `jg.Mu.Lock()`.
     - Valida que `jg.Players[id]` no exista (rechazo si duplicado).
     - `jg.Colors.Acquire()` (puede fallar si pool agotado → rechazo).
     - Crea `Player`, `Outbox chan OutboundFrame(256)`, `Stop chan struct{}`.
     - Inserta en `jg.Players[id]`.
     - **Retorna** `(*MessageObject, *Players, error)` — caller usa el puntero sin re-lookup.
   - Encola `MessageObject{Data: json(AddPlayerMsg), Event: AddPlayer, From: id}`.
   - `pkg.StartWriter(player)` lanza goroutine de escritura.
   - `SetReadDeadline` + `SetPongHandler` (keepalive pasivo).
   - Entra en loop de lectura.
5. Worker toma el mensaje con `RLock`, llama `processWorker`.
6. `EventAddPlayer` arma array de `AddPlayerMsg` con todos los jugadores.
7. `SendAllMsg` encola al outbox de cada jugador (frame **Binary**).
8. Cada `Writer` goroutine desencola y escribe con `SetWriteDeadline(2s)`.

### 3.2 `MovePlayer` (cliente → servidor)

1. Cliente envía frame con `MessageObject{Data: json(Player), From: id, Events: 1}`.
2. `Conn` parsea y encola al canal.
3. Worker toma, `processWorker` con **`jg.Mu.Lock()`** (es el único caso de mutación):
   - `EventMovement`:
     - `ConvertToPlayer(dataPlayer)` → nuevo `Player` (puede devolver nil → return).
     - Sobrescribe `jg.Players[playerID].Player`.
     - `GetAllPlayer(jg, playerID)` → array de los demás con `body.position.y -= 0.3`.
     - `json.Marshal` → `MessageObject{Data: string, Event: MovePlayer, From: id}`.
   - `SendMsg` encola al outbox del jugador emisor (frame **Text**).
4. Writer del emisor escribe con deadline.

### 3.3 `RayInteraction` y `ActionHandsPlayer`

- Mismo patrón que `AddPlayer` pero con `RLock` (no mutan estado).
- `EventRayInteraction` / `EventActionsHandsAnimation` son passthrough de `data`.
- `SendAllMsg` a todos como **Binary**.

### 3.4 Desconexión y `RemovePlayer`

1. El read loop sale por error o el cliente cierra.
2. `defer` en `Conn`:
   - `pkg.DesconnectUser(jg, c, id)`:
     - `jg.Mu.Lock()`.
     - `jg.Colors.Release(p.Color)` (con nil-guard).
     - `close(p.Stop)` → writer goroutine termina.
     - `close(p.Outbox)` → writer termina por `ok=false` en select.
     - `delete(jg.Players, id)`.
     - `jg.Mu.Unlock()`.
   - Devuelve `MessageObject{Data: "", From: id, Event: RemovePlayer}`.
3. Worker procesa y `SendAllMsg` a todos — el socket ya está cerrado, pero `safeSendOutbox` con `recover` lo absorbe (log warn).

---

## 4. Concurrencia — modelo mental

```
┌────────────┐      chan MessageObject (cap 5000)       ┌────────────┐
│ Conn loop  │ ──────────────────────────────────────►  │   Workers  │
│ (N gorout) │                                         │ (6 parale- │
└────────────┘                                         │   los)     │
                                                        └─────┬──────┘
                                                              │  RLock/Lock por evento
                                                              ▼
                                                       jg.Players map
                                                              │
                                                    Solo encolan al outbox
                                                              │
                                                              ▼
┌────────────┐      Outbox (per-conn, cap 256)         ┌────────────┐
│  Writer    │ ◄────────────────────────────────────── │   Workers  │
│ (per-conn) │                                         │            │
└─────┬──────┘                                         └────────────┘
      │  SetWriteDeadline + WriteMessage
      ▼
 *websocket.Conn
```

- **Productores de mensajes**: N goroutines `Conn` (unen al canal de la cola).
- **Consumidores de mensajes**: 6 goroutines `Worker` (paralelas, locks granulares).
- **Productores de frames**: 6 workers → N outboxes.
- **Consumidores de frames**: N goroutines `Writer` (una por socket, escribe al wire).
- **Buffer de mensajes**: 5000 (config `CBTIC_QUEUE_BUFFER`).
- **Buffer de frames**: 256 por socket (config `CBTIC_OUTBOX_SIZE`).
- **Backpressure**: si un outbox se llena, el frame se **dropea + warn** (no bloquea al worker). Si la cola de mensajes se llena, el `Conn` bloquea (lectura del socket pausa).

---

## 5. Estado compartido mutable

| Recurso | Protección | Notas |
|---|---|---|
| `jg.Players` (mapa) | `jg.Mu sync.RWMutex` | `RLock` en lecturas, `Lock` solo en `MovePlayer` y `AddUser`/`DesconnectUser` |
| `colors.Pool` | `sync.Mutex` interno | `Acquire`/`Release` thread-safe |
| `Outbox` (per-conn) | `select` con `default` | Drop-on-full + warn; `recover` para send-on-closed |
| `Stop` (per-conn) | close-once | Garantizado por mutex externo en `DesconnectUser` |
| Canal `jg.Queue` | Internamente sincronizado | OK |
| Sockets (`*websocket.Conn`) | **Una sola goroutine** (`Writer`) | Ningún otro código toca el socket |

---

## 6. Manejo de errores y recover

| Punto | Mecanismo |
|---|---|
| Worker panic | `defer recover` **dentro del for** (por mensaje) — log error + continúa |
| Worker goroutine panic | `defer recover` en `worker()` — log fatal pero la goroutine muere (las demás siguen) |
| WriteMessage falla | `SetReadDeadline(time.Now())` → cierra el socket al siguiente read |
| Outbox lleno | `select default` → drop + warn |
| Outbox cerrado | `defer recover` en `safeSendOutbox` → warn + continúa |
| SetWriteDeadline falla | Log warn + writer termina |
| ConvertToPlayer falla | Retorna `nil` → worker detecta + log warn + descarta mensaje |
| AddUser falla (id dup, color) | Retorna error → `Conn` cierra socket |

---

## 7. Configuración (env vars)

| Variable | Default | Descripción |
|---|---|---|
| `CBTIC_ADDRESS` | `:8080` | Puerto de escucha |
| `CBTIC_QUEUE_BUFFER` | `5000` | Buffer del canal de mensajes |
| `CBTIC_QUEUE_WORKERS` | `6` | Número de workers |
| `CBTIC_OUTBOX_SIZE` | `256` | Buffer del outbox por conexión |
| `CBTIC_WRITE_TIMEOUT` | `2s` | Deadline de escritura WS |
| `CBTIC_READ_TIMEOUT` | `60s` | Keepalive pasivo (read deadline) |
| `CBTIC_SHUTDOWN_TIMEOUT` | `10s` | Graceful shutdown |
| `CBTIC_LOG_LEVEL` | `info` | Niveles zerolog (debug/info/warn/error) |

---

## 8. Dependencias externas

| Dependencia | Uso |
|---|---|
| `github.com/gofiber/fiber/v2` | HTTP + routing |
| `github.com/gofiber/contrib/websocket` | Adaptador WS sobre fasthttp |
| `github.com/rs/zerolog` | Logger estructurado JSON |
| `github.com/google/uuid` (indirecta) | **No se usa** (`:id` por URL) |
| `github.com/valyala/bytebufferpool` (indirecta) | **No se usa** (pendiente F3 o B) |

---

## 9. Decisiones de diseño clave

| Decisión | Razón |
|---|---|
| Outbox por conexión (no canal global) | Escrituras no bloquean workers; un cliente lento no frena al resto |
| `Writer` goroutine por socket | Solo una goroutine toca el `*websocket.Conn` (fasthttp no es safe para escrituras concurrentes) |
| `Stop` channel para cierre | `close(p.Outbox)` desde fuera causa DATA RACE con `select` interno |
| `RLock` en lecturas, `Lock` en mutación | Hot path de `MovePlayer` sigue siendo serializado (es mutación inevitable), pero el resto es paralelo |
| Keepalive pasivo (no pinger) | `WriteControl` causa el mismo DATA RACE que `Close` (fasthttp `releaseConn`) |
| Pool HSL infinito | 4320 colores únicos es más que suficiente para VR; algoritmo simple y determinista |
| AddUser retorna `(*MessageObject, *Players, error)` | Evita race de re-lookup entre `AddUser` y `StartWriter` |
| `safeSendOutbox` con `recover` | Tolerancia a send-on-closed sin crashes |

---

## 10. Puntos de extensión futuros

- **F3**: `ConvertToJson` propaga error, `SetReadLimit` para OOM, nil guards adicionales.
- **A3**: Benchmark sintético con `pprof` para identificar hot path real.
- **B**: Migración a `coder/websocket` + `net/http` (solo si A3 lo justifica). La interfaz `Conn` en `B1` permitirá swap sin tocar el dominio.

---

## 11. Referencias

- Contrato de red: [`../SPEC.md`](../SPEC.md)
- Plan por fases: [`./ROADMAP.md`](./ROADMAP.md)
- Auditoría + resolución: [`./AUDIT.md`](./AUDIT.md)
- Log cronológico: [`./CHANGELOG.md`](./CHANGELOG.md)
