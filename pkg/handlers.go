package pkg

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"slices"
	"time"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/gofiber/contrib/websocket"
)

var (
	// Lista completa de colores hexadecimales
	colors = []string{
		"#FF5733",
		"#33FF57",
		"#3357FF",
		"#F1C40F",
		"#9B59B6",
		"#1ABC9C",
		"#E74C3C",
		"#8E44AD",
		"#2ECC71",
		"#3498DB",
		// …añade todos los que necesites
	}

	// Copia de los colores que aún no se han usado
	remaining []string
	// muIdentify sync.Mutex
)

func init() {
	// Semilla para que rand.Intn sea verdaderamente aleatorio
	rand.Seed(time.Now().UnixNano())
	// Inicializamos remaining con todos los colores
	remaining = make([]string, len(colors))
	copy(remaining, colors)
}

func IdentifyPlayer() (string, error) {
	if len(remaining) == 0 {
		return "", fmt.Errorf("no quedan colores únicos disponibles")
	}

	// Escogemos un índice aleatorio dentro de remaining
	idx := rand.Intn(len(remaining))
	color := remaining[idx]

	// Lo eliminamos de remaining para que no se repita
	remaining = slices.Delete(remaining, idx, idx+1)

	return color, nil
}

func DesconnectUser(jg *typegame.JobGame, c *websocket.Conn, id string) typegame.MessageObject {
	// cerrar conexion
	c.Close()
	// eliminar el usuario
	jg.Mu.Lock()
	delete(jg.Players, id)
	// delete(jg.Conn, id)
	// delete(jg.Players, id)
	jg.Mu.Unlock()

	return typegame.MessageObject{
		Data:  "",
		From:  id,
		Event: config.RemovePlayer,
	}
}

func AddUser(jg *typegame.JobGame, c *websocket.Conn, idPlayer string) *typegame.MessageObject {
	jg.Mu.Lock()
	defer jg.Mu.Unlock()

	// Crea el nuevo objeto jugador
	color, _ := IdentifyPlayer()
	py := &typegame.Player{}
	player := &typegame.Players{
		Id:     idPlayer,
		Color:  color,
		Conn:   c,
		Player: py,
	}

	data := ConvertToJson(&typegame.AddPlayerMsg{
		Id:    idPlayer,
		Color: color,
	})

	// Así es como agregas correctamente al mapa
	jg.Players[idPlayer] = player

	return ConvertToMessageObject(string(data), idPlayer, config.AddPlayer)
}

func ConvertToMessageObject(data string, from string, event config.EventServer) *typegame.MessageObject {
	return &typegame.MessageObject{
		Data:  data,
		From:  from,
		Event: event,
	}
}

func ByteToMessageObject(msg []byte) (typegame.MessageObject, error) {
	var message typegame.MessageObject
	err := json.Unmarshal(msg, &message)
	if err != nil {
		log.Printf("Error al deserializar el mensaje: %v", err)
		return typegame.MessageObject{}, err
	}
	return message, nil
}

func ConvertToJson(data any) []byte {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error al convertir a JSON: %v", err)
		return nil
	}
	return dataJSON
}

func SendAllMsg(jg *typegame.JobGame, msg []byte) {
	// for _, c := range jg.Conn {
	for _, c := range jg.Players {
		if err := c.Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			log.Printf("Error enviando: %v", err)
		}
	}
}

func SendMsg(jg *typegame.JobGame, idPlayer string, msg []byte) {
	py, exists := jg.Players[idPlayer]

	if !exists {
		return
	}

	if msg == nil {
		log.Printf("MSG no se puede enviar por que es null: %v", msg)
		return
	}

	err := py.Conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		log.Printf("Error escribiendo en WebSocket: %v", err)
	}
}

func SendMsgAllWithout(jg *typegame.JobGame, idPlayer string, msg []byte) {
	for index, c := range jg.Players {
		// playerCurrent := jg.Players[index]
		playerCurrent := jg.Players[index]

		if playerCurrent.Id != idPlayer {
			if err := c.Conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				log.Printf("Error enviando: %v", err)
			}
		}

	}
}
