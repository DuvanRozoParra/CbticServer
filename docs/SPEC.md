# SPEC — CbticServer Wire Contract v1

> **🔒 CONTRATOS CONGELADOS** — Los JSON de entrada y salida definidos en este documento son **inmutables** salvo acuerdo explícito con el cliente Unity (endel/NativeWebSocket). Cualquier refactor interno de performance o escalabilidad debe preservar exactamente los bytes descritos aquí. Todo lo demás (arquitectura, locks, workers, paquetes, logs) puede modificarse con aprobación caso por caso.

| Campo | Valor |
|---|---|
| Versión del contrato | **1.1** |
| Cliente objetivo | Unity + endel/NativeWebSocket |
| Framework servidor | Go + Fiber v2 + gofiber/contrib/websocket |
| Transporte | WebSocket RFC 6455, sin subprotocolos |
| Puerto | TCP `8080` |

---

## 1. Endpoint y transporte

| Ítem | Valor |
|---|---|
| Método / Ruta | `GET /api/v1/ws/:id` |
| `:id` | Identificador del jugador (string libre) |
| Upgrade | RFC 6455 estándar (`Upgrade: websocket`) |
| Subprotocolos | **Ninguno** (no se negocia `Sec-WebSocket-Protocol`) |
| Middleware | `ConnectWebsocket` rechaza con `426 Upgrade Required` si no es upgrade válido |

### 🔒 No negociable
- Ruta exacta, puerto, ausencia de subprotocolos.

---

## 2. Envoltura canónica de mensajes

Todo frame de aplicación (entrada y salida) usa la misma estructura JSON:

```json
{
  "data":   "<string>",
  "from":   "<string>",
  "events": <int>
}
```

### 🔒 Reglas inmutables (gotchas a preservar)

1. El campo evento se llama **`events`** (plural) — `internal/typesGame/conn.go:10`.
2. `data` **SIEMPRE** es un `string` — nunca objeto ni array directo.
3. `data` puede contener **JSON serializado dentro** (doble capa) en varios eventos.
4. `events` es un **entero** (no string).
5. Sin `omitempty`: campos vacíos se envían como `""` o `0`.

### Tipos de frame WebSocket por evento (estables)

| Evento saliente | Destino | Opcode | Frame |
|---|---|---|---|
| `AddPlayer` (3) | broadcast | 2 | **Binary** |
| `MovePlayer` (1) | broadcast (excepto emisor) | 1 | **Text** |
| `RayInteraction` (0) | broadcast | 2 | **Binary** |
| `ActionHandsPlayer` (2) | broadcast | 2 | **Binary** |
| `RemovePlayer` (5) | broadcast | 2 | **Binary** |
| `ServerShutdown` (7) | broadcast | 1 | **Text** |

> ⚠️ El servidor usa **dos tipos de frame distintos**. El cliente Unity debe manejar ambos (NativeWebSocket entrega `byte[]` y/o string según el opcode).

---

## 3. Catálogo de eventos

Definido en `internal/config/config.go:11-22`.

| Valor | Nombre | Sentido |
|---|---|---|
| `0` | `RayInteraction` | cliente → servidor |
| `1` | `MovePlayer` | cliente → servidor |
| `2` | `ActionHandsPlayer` | cliente → servidor |
| `3` | `AddPlayer` | servidor → cliente |
| `4` | `UpdatePlayer` | (reservado, sin handler) |
| `5` | `RemovePlayer` | servidor → cliente |
| `6` | `IdentifyPlayer` | (reservado, comentado) |
| `7` | `ServerShutdown` | servidor → cliente |

---

## 4. Tipos de datos compartidos

Definidos en `internal/typesGame/player.go`.

### 4.1 `Vector3`
```json
{ "x": 0.0, "y": 0.0, "z": 0.0 }
```
Tres `float64`.

### 4.2 `Quaternion`
```json
{ "x": 0.0, "y": 0.0, "z": 0.0, "w": 1.0 }
```
Cuatro `float64` (orden x, y, z, w).

### 4.3 `BodyPart`
```json
{
  "position": { "x": 0.0, "y": 0.0, "z": 0.0 },
  "rotation": { "x": 0.0, "y": 0.0, "z": 0.0, "w": 1.0 }
}
```

### 4.4 `Player`
```json
{
  "id": "<string>",
  "head":      { "position": {...}, "rotation": {...} },
  "body":      { "position": {...}, "rotation": {...} },
  "handLeft":  { "position": {...}, "rotation": {...} },
  "handRight": { "position": {...}, "rotation": {...} }
}
```

### 4.5 `AddPlayerMsg`
```json
{ "id": "<string>", "color": "#RRGGBB" }
```

---

## 5. Contratos por evento (IN/OUT exactos)

### 5.1 `RayInteraction` (events = 0)

**IN** (cliente → servidor):
```json
{ "data": "<string arbitrario>", "from": "<playerId>", "events": 0 }
```

**OUT** (broadcast a TODOS los jugadores conectados, frame **Binary**):
```json
{ "data": "<mismo string recibido>", "from": "<playerId emisor>", "events": 0 }
```

> El servidor actúa como **relay**: no interpreta ni transforma `data`.

---

### 5.2 `MovePlayer` (events = 1)

**IN** (cliente → servidor): `data` es un **string** que contiene un `Player` serializado en JSON.
```json
{
  "data": "{\"id\":\"\",\"head\":{...},\"body\":{...},\"handLeft\":{...},\"handRight\":{...}}",
  "from": "<playerId>",
  "events": 1
}
```

**OUT** (broadcast a **todos EXCEPTO el emisor**, frame **Text**):
`data` = string con **un único `Player` JSON** correspondiente al jugador que se movió (el emisor), con `body.position.y -= 0.3` aplicado (`internal/core/players/events.go`).
```json
{
  "data": "{\"id\":\"<emisorId>\",\"head\":{...},\"body\":{...},\"handLeft\":{...},\"handRight\":{...}}",
  "from": "<emisorId>",
  "events": 1
}
```

> v1.1: cambio de modelo. Antes v1.0 enviaba un array de todos los jugadores al propio emisor. Ahora broadcastea la posición del emisor a los demás. Esto reduce el trabajo de CPU de O(N²) a O(N) por frame y mejora la latencia de updates. El cliente debe mantener un mapa local de posiciones de otros jugadores.

---

### 5.3 `ActionHandsPlayer` (events = 2)

**IN**:
```json
{ "data": "<string arbitrario>", "from": "<playerId>", "events": 2 }
```

**OUT** (broadcast a TODOS, frame **Binary**): retransmisión literal.
```json
{ "data": "<mismo string>", "from": "<playerId>", "events": 2 }
```

---

### 5.4 `AddPlayer` (events = 3)

**Disparador:** automático al establecer el WS (`internal/network/ws/handlers/conn.go:14-15`).

**OUT A — evento interno encolado** (con `data` = string de `AddPlayerMsg` del nuevo jugador):
```json
{ "data": "{\"id\":\"<nuevoId>\",\"color\":\"#RRGGBB\"}", "from": "<nuevoId>", "events": 3 }
```

**OUT B — broadcast a TODOS, frame **Binary**:** (lo que efectivamente reciben los clientes)
`data` = string con un **array JSON de `AddPlayerMsg`** conteniendo a **todos** los jugadores actuales (`internal/core/players/handlers.go:20-33`).
```json
{
  "data": "[{\"id\":\"<id1>\",\"color\":\"#FF5733\"},{\"id\":\"<id2>\",\"color\":\"#33FF57\"}]",
  "from": "<nuevoId>",
  "events": 3
}
```

> **Paleta de colores actual** (`internal/colors/pool.go`): pool HSL infinito (~4320 colores únicos, hue áureo + 4 capas de L × 3 capas de S). Formato `#RRGGBB`. Si se amplía, se mantiene el formato.

---

### 5.5 `RemovePlayer` (events = 5)

**Disparador:** automático al desconectarse el socket (`internal/network/ws/handlers/conn.go:20-21`).

**OUT** (broadcast a TODOS, frame **Binary**): `data` **vacío**.
```json
{ "data": "", "from": "<playerId saliente>", "events": 5 }
```

---

### 5.6 `ServerShutdown` (events = 7)

**Disparador:** el servidor recibe `SIGTERM`/`SIGINT` (`cmd/main.go`). Se envía **antes** de cerrar las conexiones, dando ~500 ms a los clientes para reaccionar.

**OUT** (broadcast a TODOS, frame **Text**):
```json
{ "data": "server_shutdown", "from": "server", "events": 7 }
```

> El cliente Unity debe interpretar este evento como "servidor reiniciándose" y mostrar un mensaje al usuario antes de que la conexión se cierre (~500 ms después).

---

## 6. Compatibilidad con endel/NativeWebSocket (Unity)

| Requisito de NativeWebSocket | Estado actual |
|---|---|
| WebSocket RFC 6455 estándar | ✅ |
| Sin subprotocolos obligatorios | ✅ |
| Soporte de frames Text y Binary | ⚠️ El servidor usa ambos — Unity debe manejar los dos |
| Reconexión manual (la lib no auto-reconecta) | ✅ Tolerada |
| Pings/pongs de control | Manejados por el stack subyacente |

**Notas de robustez (sin romper contrato):**
- Se pueden agregar `SetReadDeadline`/pings a nivel servidor para limpiar zombis sin alterar el wire de aplicación.
- NativeWebSocket responde pongs de control automáticamente, sin que la app Unity haga nada.

---

## 7. Reglas para refactors

### 🔒 NO modificar sin aprobación explícita y aviso previo al cliente Unity
- Cualquier JSON descrito en §5.
- Endpoints (§1), puerto, subprotocolos.
- Tipos de frame por evento (§2).
- Destinatarios (broadcast vs. solo emisor) por evento.
- La transformación `body.position.y -= 0.3` en `MovePlayer`.
- El conjunto actual de colores hex y su formato.
- La regla de `data` siempre como string (incluso cuando contiene JSON).

### ✅ Modificable con aprobación caso por caso
- Arquitectura interna (locks, workers, rooms, particionamiento).
- Pooling de buffers, deadlines internos, logging estructurado.
- Generación server-side de IDs (afecta wire — coordinar con Unity antes).
- Graceful shutdown, heartbeats.
- Nuevos eventos con valores `>= 7` (no rompe contrato de los existentes).

---

## 8. Versionado

Cualquier cambio en JSON, frames, endpoints o destinatarios por evento requiere:
1. Bump de versión del contrato (v1.0 → v1.1, v2.0, etc.).
2. Nota en `docs/CHANGELOG.md` (cuando exista).
3. Aviso al equipo Unity **antes** de desplegar.
4. Período de coexistencia si el cambio es incompatible.

---

## 9. Glosario rápido

- **Wire contract**: el formato exacto de los bytes que cruzan el cable (WebSocket + JSON).
- **Frame Binary**: opcode `0x2` en RFC 6455. NativeWebSocket lo entrega como `byte[]`.
- **Frame Text**: opcode `0x1`. NativeWebSocket lo entrega como `string` o `byte[]` según el helper usado.
- **Doble capa JSON**: `data` es un string que contiene JSON serializado (`"{\"k\":\"v\"}"`).
