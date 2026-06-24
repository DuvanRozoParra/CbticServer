package typegame

import (
	"sync"

	"github.com/DuvanRozoParra/servercbtic/internal/colors"
	"github.com/gofiber/contrib/websocket"
)

type Players struct {
	Conn   *websocket.Conn
	Player *Player
	Id     string
	Color  string
	Outbox chan OutboundFrame
	Stop   chan struct{}
}

type OutboundFrame struct {
	Opcode int
	Data   []byte
}

type JobGame struct {
	Players map[string]*Players
	Queue   chan MessageObject
	Colors  *colors.Pool
	Mu      sync.RWMutex
}
