package queue

import (
	"log"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	"github.com/DuvanRozoParra/servercbtic/internal/core/players"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/DuvanRozoParra/servercbtic/pkg"
)

func StartWorkers(workerSize int, jg *typegame.JobGame) {
	for i := range workerSize {
		go worker(i+1, jg)
	}
}

func worker(idWorker int, jg *typegame.JobGame) {
	log.Printf("Worker %d iniciado", idWorker)
	for {
		msg := <-jg.Queue
		processWorker(jg, msg)
	}
}

func processWorker(jg *typegame.JobGame, msg typegame.MessageObject) {
	jg.Mu.Lock()
	defer jg.Mu.Unlock()

	player := players.GetPlayer(jg, msg.From)

	if player == nil && msg.Event != config.RemovePlayer {
		return
	}

	switch msg.Event {
	case config.AddPlayer:
		interaction := players.EventAddPlayer(jg, msg.From)
		json := pkg.ConvertToJson(interaction)
		pkg.SendAllMsg(jg, json)

	case config.MovePlayer:
		playersArray := players.EventMovement(jg, msg.From, msg.Data)
		json := pkg.ConvertToJson(playersArray)
		pkg.SendMsg(jg, msg.From, json)

	case config.RayInteraction:
		interaction := players.EventRayInteraction(jg, player.Id, msg.Data)
		json := pkg.ConvertToJson(interaction)
		pkg.SendAllMsg(jg, json)

	case config.ActionHandsPlayer:
		interaction := players.EventActionsHandsAnimation(jg, player.Id, msg.Data)
		json := pkg.ConvertToJson(interaction)
		pkg.SendAllMsg(jg, json)

	case config.RemovePlayer:
		interaction := players.EventRemovePlayer(jg, msg.From)
		json := pkg.ConvertToJson(interaction)
		pkg.SendAllMsg(jg, json)

		/*
			case config.IdentifyPlayer:
								json := pkg.ConvertToJson(msg)
								pkg.SendAllMsg(jg, json)

		*/

	default:
		log.Printf("Evento desconocido: %v", msg.Event)
	}

}
