package queue

import (
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
)

func New(bufferSize int) *typegame.JobGame {
	return &typegame.JobGame{
		// Conn:    make(map[string]*websocket.Conn),
		// Players: make(map[string]*typegame.Player),
		Players: make(map[string]*typegame.Players),
		Queue:   make(chan typegame.MessageObject, bufferSize),
	}
}
