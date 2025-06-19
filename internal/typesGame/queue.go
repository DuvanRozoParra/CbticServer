package typegame

import (
	"sync"

	"github.com/gofiber/contrib/websocket"
)

type Players struct {
	Conn   *websocket.Conn
	Player *Player
	Id     string
	Color  string
}

type JobGame struct {
	// Conn    map[string]*websocket.Conn
	// Players map[string]*Player
	Players map[string]*Players
	Queue   chan MessageObject
	Mu      sync.RWMutex
}
