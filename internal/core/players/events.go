package players

import (
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
)

func EventMovement(jg *typegame.JobGame, playerID string, dataPlayer string) *typegame.Player {
	currentPlayer := ConvertToPlayer(dataPlayer)
	if currentPlayer == nil {
		return nil
	}
	p, exists := jg.Players[playerID]
	if !exists || p == nil {
		return nil
	}
	p.Player = currentPlayer

	result := *currentPlayer
	result.Id = playerID
	result.Body.Position.Y -= 0.3
	return &result
}

func EventAddPlayer(jg *typegame.JobGame, playerId string) []typegame.AddPlayerMsg {
	return GetAllPlayerId(jg, playerId)
}

func EventRayInteraction(jg *typegame.JobGame, playerId string, dataEvent string) string {
	return dataEvent
}

func EventActionsHandsAnimation(jg *typegame.JobGame, playerId string, dataAction string) string {
	return dataAction
}

func EventRemovePlayer(jg *typegame.JobGame, playerId string) string {
	return ""
}
