package queue

import (
	"github.com/DuvanRozoParra/servercbtic/internal/colors"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
)

func New(bufferSize int) *typegame.JobGame {
	return &typegame.JobGame{
		Players: make(map[string]*typegame.Players),
		Queue:   make(chan typegame.MessageObject, bufferSize),
		Colors:  colors.NewPool(),
	}
}
