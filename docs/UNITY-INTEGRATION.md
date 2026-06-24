# Guía de Integración — Cliente Unity

> **Versión del servidor:** v1.1.0 (rama `lts`)
> **Wire contract:** SPEC v1.1
> **Endpoint:** `ws://<host>:8080/api/v1/ws/<playerId>`

Este documento es **autocontenido**. Un desarrollador de Unity puede implementar el cliente multiplayer completo leyendo solo este archivo, sin necesidad de revisar el código del servidor.

---

## 1. Conexión

### URL

```
ws://<IP_DEL_SERVIDOR>:8080/api/v1/ws/<playerId>
```

- `<playerId>` es un identificador **único** por jugador (string). Lo define el cliente. Ejemplo: `player-7`, `user_abc123`.
- Si otro jugador ya usa el mismo ID, el servidor **echa la conexión vieja** y acepta la nueva (auto-kick). No hay que esperar.

### Health check (opcional)

```
GET http://<host>:8080/api/health
```

Respuesta:
```json
{ "status": "ok", "players": 42, "queue": 3 }
```

Útil para verificar que el servidor está vivo antes de conectar el WebSocket.

### Conexión con NativeWebSocket (C#)

```csharp
using NativeWebSocket;
using UnityEngine;

public class NetworkClient : MonoBehaviour
{
    WebSocket ws;
    public string playerId = "player-1";
    public string serverHost = "192.168.1.100";

    async void Start()
    {
        ws = new WebSocket($"ws://{serverHost}:8080/api/v1/ws/{playerId}");

        ws.OnOpen    += () => Debug.Log("Conectado al servidor");
        ws.OnError   += (e) => Debug.LogError($"WS error: {e}");
        ws.OnClose   += (e) => Debug.Log($"WS cerrado: {e}");
        ws.OnMessage += OnMessageReceived;

        await ws.Connect();
    }

    void Update()
    {
        // Procesar la cola de mensajes entrantes (necesario en Unity)
#if !UNITY_WEBGL || UNITY_EDITOR
        ws?.DispatchMessageQueue();
#endif
    }

    async void OnApplicationQuit()
    {
        if (ws != null)
            await ws.Close();
    }
}
```

---

## 2. Estructura de mensajes

**Todos** los mensajes (entrantes y salientes) usan la misma estructura JSON:

```json
{
  "data": "<contenido del evento, serializado como string>",
  "from": "<playerId del emisor>",
  "events": <número de evento>
}
```

| Campo | Tipo | Descripción |
|---|---|---|
| `data` | `string` | El payload del evento. Su contenido depende del evento (ver catálogo). Es un **string** que a su vez puede contener JSON. |
| `from` | `string` | El ID del jugador que originó el mensaje. Para eventos generados por el servidor, puede ser `"server"`. |
| `events` | `int` | Código del evento (ver catálogo). Define cómo interpretar `data`. |

### Clase C# equivalente

```csharp
[System.Serializable]
public class MessageObject
{
    public string data;    // payload (string que puede contener JSON)
    public string from;    // ID del emisor
    public int events;     // código de evento
}
```

---

## 3. Catálogo de eventos

### Eventos que el cliente ENVÍA al servidor

| `events` | Nombre | Opcode | `data` (qué enviar) |
|---|---|---|---|
| `1` | `MovePlayer` | **Text** | JSON string de tu `Player` (posiciones de head/body/manos) |
| `0` | `RayInteraction` | **Binary** | String libre (passthrough, el servidor no lo procesa) |
| `2` | `ActionHandsPlayer` | **Binary** | String libre (passthrough, el servidor no lo procesa) |

### Eventos que el cliente RECIBE del servidor

| `events` | Nombre | Opcode | `data` (qué recibir) | Origen |
|---|---|---|---|---|
| `1` | `MovePlayer` | **Text** | JSON string de **un** `Player` (con `y -= 0.3`) | Otro jugador se movió |
| `3` | `AddPlayer` | **Binary** | JSON string con **array** de `[{id, color}, ...]` | Lista de jugadores actuales |
| `5` | `RemovePlayer` | **Binary** | `""` (vacío) | Un jugador se desconectó |
| `0` | `RayInteraction` | **Binary** | String (passthrough del emisor original) | Otro jugador hizo raycast |
| `2` | `ActionHandsPlayer` | **Binary** | String (passthrough del emisor original) | Otro jugador animó manos |
| `7` | `ServerShutdown` | **Text** | `"server_shutdown"` | El servidor se está reiniciando |

> **Eventos reservados (ignorar):** `4` (UpdatePlayer) y `6` (IdentifyPlayer) no se usan actualmente.

---

## 4. Tipos de datos

### `Player`

Representa el avatar completo de un jugador con tracking de VR.

```json
{
  "id": "player-1",
  "head":  { "position": {"x":0, "y":1.7, "z":0}, "rotation": {"x":0, "y":0, "z":0, "w":1} },
  "body":  { "position": {"x":0, "y":1.0, "z":0}, "rotation": {"x":0, "y":0, "z":0, "w":1} },
  "handLeft":  { "position": {"x":-0.3, "y":1.0, "z":0}, "rotation": {"x":0, "y":0, "z":0, "w":1} },
  "handRight": { "position": {"x":0.3, "y":1.0, "z":0}, "rotation": {"x":0, "y":0, "z":0, "w":1} }
}
```

### Clases C#

```csharp
[System.Serializable]
public class Vector3Ser
{
    public float x, y, z;
}

[System.Serializable]
public class QuaternionSer
{
    public float x, y, z, w;
}

[System.Serializable]
public class BodyPart
{
    public Vector3Ser position;
    public QuaternionSer rotation;
}

[System.Serializable]
public class Player
{
    public string id;
    public BodyPart head;
    public BodyPart body;
    public BodyPart handLeft;
    public BodyPart handRight;
}

// Usado solo en el evento AddPlayer (3)
[System.Serializable]
public class AddPlayerMsg
{
    public string id;
    public string color;   // formato "#RRGGBB", ej: "#FF5C3A"
}

[System.Serializable]
public class AddPlayerMsgList
{
    public AddPlayerMsg[] players;   // wrapper para arrays (JsonUtility no deserializa arrays top-level)
}
```

> **Nota sobre `JsonUtility`:** Unity no puede deserializar un array JSON top-level (`[{...},{...}]`). Hay que envolverlo en un objeto o usar `Newtonsoft.Json` (recomendado para arrays).

---

## 5. Ciclo de vida del jugador

### Conexión inicial

Al conectar, el servidor responde con **un evento `AddPlayer` (3)** dirigido al nuevo jugador, conteniendo la **lista de todos los jugadores ya conectados** (con sus colores):

```json
{
  "data": "[{\"id\":\"player-2\",\"color\":\"#FF5C3A\"},{\"id\":\"player-3\",\"color\":\"#3AFF8C\"}]",
  "from": "player-1",
  "events": 3
}
```

El cliente debe instanciar los avatares de esos jugadores.

Simultáneamente, el servidor envía **a todos los demás jugadores** un evento `AddPlayer` (3) con el nuevo jugador en la lista:

```json
{
  "data": "[{\"id\":\"player-1\",\"color\":\"#5C8CFF\"}]",
  "from": "player-1",
  "events": 3
}
```

### Movimiento continuo (120 Hz)

El cliente envía su `Player` completo ~120 veces por segundo:

**Enviar (cliente → servidor):**
```json
{
  "data": "{\"id\":\"\",\"head\":{\"position\":{\"x\":0,\"y\":1.7,\"z\":0},...}}",
  "from": "player-1",
  "events": 1
}
```

> El campo `id` dentro de `data` puede ir vacío; el servidor lo reemplaza con el `from`.

**Recibir (servidor → todos los demás):**
```json
{
  "data": "{\"id\":\"player-1\",\"head\":{...},\"body\":{\"position\":{\"x\":0,\"y\":1.4,\"z\":0},...}}",
  "from": "player-1",
  "events": 1
}
```

> **Importante:** el servidor aplica `body.position.y -= 0.3` antes de reenviar. Esto ajusta la altura del avatar para que se vea bien desde la perspectiva de otros jugadores. No hay que aplicarlo en el cliente.

### Desconexión

Cuando un jugador se desconecta, el servidor envía `RemovePlayer` (5) a todos los demás:

```json
{
  "data": "",
  "from": "player-2",
  "events": 5
}
```

El cliente debe destruir el avatar de `player-2`.

---

## 6. Evento `ServerShutdown` (7)

Cuando el servidor se reinicia (SIGTERM/SIGINT), envía a todos:

```json
{
  "data": "server_shutdown",
  "from": "server",
  "events": 7
}
```

**~500ms después**, el servidor cierra la conexión. El cliente debe:

1. Mostrar un mensaje al usuario: "El servidor se está reiniciando..."
2. Detener el envío de MovePlayer
3. Intentar reconectar después de unos segundos

```csharp
void HandleServerShutdown()
{
    Debug.Log("Servidor reiniciándose");
    // Mostrar UI de "reconectando..."
    // Cancelar envío de MovePlayer
    // Intentar reconectar tras 3-5 segundos
}
```

---

## 7. Rate limit (importante)

El servidor limita cada conexión a **200 mensajes por segundo** (`CBTIC_INGRESS_RATE`). Los mensajes que excedan este límite se **descartan silenciosamente**.

- A **120 Hz**, `MovePlayer` consume 120 de los 200 slots.
- Quedan ~80 slots/s para `RayInteraction` y `ActionHandsPlayer`.
- Si el cliente envía ráfagas (ej. múltiples eventos en un frame), puede alcanzar el límite.

> Si el cliente envía más de 200 msg/s constantemente, **algunos mensajes se perderán** y el movimiento se verá entrecortado a los demás jugadores. Mantener el bucle de envío a 120 Hz o menos.

---

## 8. Snippets de código C#

### Enviar `MovePlayer`

```csharp
async void SendMovePlayer(Player myPlayer)
{
    string playerJson = JsonUtility.ToJson(myPlayer);
    var msg = new MessageObject
    {
        data = playerJson,
        from = playerId,
        events = 1   // MovePlayer
    };
    string wrapperJson = JsonUtility.ToJson(msg);
    await ws.SendText(wrapperJson);
}
```

### Parsear mensajes entrantes

```csharp
void OnMessageReceived(byte[] bytes)
{
    // El servidor usa opcodes Text(1) y Binary(2).
    // NativeWebSocket entrega ambos como byte[].
    string rawJson = System.Text.Encoding.UTF8.GetString(bytes);

    MessageObject msg = JsonUtility.FromJson<MessageObject>(rawJson);

    switch (msg.events)
    {
        case 1:  // MovePlayer — OTRO jugador se movió
            Player otherPlayer = JsonUtility.FromJson<Player>(msg.data);
            UpdateRemoteAvatar(msg.from, otherPlayer);
            break;

        case 3:  // AddPlayer — lista de jugadores
            // JsonUtility no deserializa arrays top-level.
            // Envolver o usar Newtonsoft:
            // string wrapped = "{\"players\":" + msg.data + "}";
            // AddPlayerMsgList list = JsonUtility.FromJson<AddPlayerMsgList>(wrapped);
            SpawnPlayers(msg.data);
            break;

        case 5:  // RemovePlayer
            DespawnPlayer(msg.from);
            break;

        case 7:  // ServerShutdown
            HandleServerShutdown();
            break;

        case 0:  // RayInteraction (passthrough)
        case 2:  // ActionHandsPlayer (passthrough)
            HandlePassthrough(msg.events, msg.from, msg.data);
            break;
    }
}
```

### Enviar `RayInteraction` o `ActionHandsPlayer`

```csharp
async void SendRayInteraction(string rayData)
{
    var msg = new MessageObject
    {
        data = rayData,
        from = playerId,
        events = 0   // RayInteraction
    };
    string wrapperJson = JsonUtility.ToJson(msg);
    await ws.SendText(wrapperJson);
}
```

### Bucle de envío a 120 Hz

```csharp
float sendInterval = 1f / 120f;   // ~8.3ms
float timer = 0f;

void Update()
{
    ws?.DispatchMessageQueue();

    timer += Time.deltaTime;
    if (timer >= sendInterval)
    {
        timer = 0f;
        SendMovePlayer(BuildPlayerFromRig());
    }
}

Player BuildPlayerFromRig()
{
    return new Player
    {
        id = "",   // el servidor lo reemplaza
        head = ToBodyPart(headTransform),
        body = ToBodyPart(bodyTransform),
        handLeft = ToBodyPart(leftHandTransform),
        handRight = ToBodyPart(rightHandTransform)
    };
}

BodyPart ToBodyPart(Transform t)
{
    return new BodyPart
    {
        position = new Vector3Ser { x = t.position.x, y = t.position.y, z = t.position.z },
        rotation = new QuaternionSer { x = t.rotation.x, y = t.rotation.y, z = t.rotation.z, w = t.rotation.w }
    };
}
```

---

## 9. Manejo de errores y reconexión

| Situación | Comportamiento del servidor | Acción del cliente |
|---|---|---|
| JSON malformado | Se descarta el mensaje (warn en server log) | Nada, corregir el serializador |
| `data` no es un Player válido en MovePlayer | Se descarta (warn en server log) | Nada, validar el payload |
| Reconexión con mismo ID | Auto-kick de la conexión vieja | Reconexión inmediata funciona |
| Cliente lento (no lee) | El outbox se llena, se droppean mensajes | Nada crítico, puede haber lag |
| Mensaje > 1 MB | Se cierra la conexión | No enviar payloads gigantes |
| Servidor reiniciando | Evento `7` + cierre tras 500ms | Mostrar UI de reconexión |
| Sin actividad 60s | Se cierra por timeout | Implementar heartbeat o enviar MovePlayer |

---

## 10. Resumen rápido

```
Conectar:    ws://host:8080/api/v1/ws/<id>
Enviar pos:  { data: "<Player JSON>", from: "<id>", events: 1 }
Recibir pos: { data: "<Player JSON>", from: "<otro>", events: 1 }   // y -= 0.3
Nuevo player: { data: "[{id,color}]", from: "<id>", events: 3 }
Player sale: { data: "", from: "<id>", events: 5 }
Server off:  { data: "server_shutdown", from: "server", events: 7 }
Rate limit:  200 msg/s máximo
Frecuencia:  120 Hz recomendado para MovePlayer
```

---

## 11. Dependencias Unity

| Paquete | Uso | Notas |
|---|---|---|
| [NativeWebSocket](https://github.com/endel/NativeWebSocket) | Conexión WebSocket | Funciona en WebGL y standalone |
| `UnityEngine.JsonUtility` | Serialización | No deserializa arrays top-level; usar wrapper o Newtonsoft |
| [Newtonsoft.Json](https://www.newtonsoft.com/json) (opcional) | Arrays top-level en AddPlayer | Recomendado si JsonUtility falla con `[{...}]` |

---

*Generado para CbticServer v1.1.0 — rama `lts` — tag `v1.1.0`*
