package handlers

import (
	"log"

	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/DuvanRozoParra/servercbtic/pkg"
	"github.com/gofiber/contrib/websocket"
)

func Conn(c *websocket.Conn, jg *typegame.JobGame) {
	id := c.Params("id")

	msgAddUser := pkg.AddUser(jg, c, id)
	jg.Queue <- *msgAddUser

	log.Printf("AGREGADO JUGADOR: %+v", id)

	defer func() {
		msgDelete := pkg.DesconnectUser(jg, c, id)
		jg.Queue <- msgDelete

		log.Printf("JUGADOR ELIMINADO: %+v", id)
	}()

	var (
		// mt  int
		msg []byte
		err error
	)

	for {
		if _, msg, err = c.ReadMessage(); err != nil {
			log.Println("read:", err)
			break
		}

		message, err := pkg.ByteToMessageObject(msg)
		if err != nil {
			log.Println("Error al deserializar el mensaje:", err)
			continue
		}

		// if message.Event != config.MovePlayer {
		// log.Printf("recv: %s", msg)
		// }

		jg.Queue <- message
	}
}
