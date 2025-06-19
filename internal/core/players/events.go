package players

import (
	"encoding/json"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/DuvanRozoParra/servercbtic/pkg"
)

func EventMovement(jg *typegame.JobGame, playerID string, dataPlayer string) *typegame.MessageObject {
	currentPlayer := ConvertToPlayer(dataPlayer)
	jg.Players[playerID].Player = currentPlayer

	allPlayers := GetAllPlayer(jg, playerID)
	jsonBytes, _ := json.Marshal(allPlayers)

	return pkg.ConvertToMessageObject(string(jsonBytes), playerID, config.MovePlayer)
}

func EventAddPlayer(jg *typegame.JobGame, playerId string) *typegame.MessageObject {
	allIds := GetAllPlayerId(jg, playerId)
	jsonBytes, _ := json.Marshal(allIds)
	return pkg.ConvertToMessageObject(string(jsonBytes), playerId, config.AddPlayer)
}

func EventRayInteraction(jg *typegame.JobGame, playerId string, dataEvent string) *typegame.MessageObject {
	return pkg.ConvertToMessageObject(dataEvent, playerId, config.RayInteraction)
}

func EventActionsHandsAnimation(jg *typegame.JobGame, playerId string, dataAction string) *typegame.MessageObject {
	return pkg.ConvertToMessageObject(dataAction, playerId, config.ActionHandsPlayer)
}

func EventRemovePlayer(jg *typegame.JobGame, playerId string) *typegame.MessageObject {
	return pkg.ConvertToMessageObject("", playerId, config.RemovePlayer)
}
