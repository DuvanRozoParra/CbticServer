package players

import (
	"encoding/json"
	"log"

	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
)

func GetPlayer(jg *typegame.JobGame, idPlayer string) *typegame.Player {
	player, exists := jg.Players[idPlayer]
	if !exists {
		log.Printf("Jugador no encontrado: %s", idPlayer)
		return nil
	}

	return player.Player
}

func GetAllPlayerId(jg *typegame.JobGame, idPlayer string) []typegame.AddPlayerMsg {
	playersIds := make([]typegame.AddPlayerMsg, 0, len(jg.Players))

	for _, p := range jg.Players {
		// if p.Id != idPlayer {
		playersIds = append(playersIds, typegame.AddPlayerMsg{
			Id:    p.Id,
			Color: p.Color,
		})
		// }
	}

	return playersIds
}

func GetAllPlayer(jg *typegame.JobGame, idPlayer string) []typegame.Player {
	playersCopy := make([]typegame.Player, 0, len(jg.Players))

	for _, p := range jg.Players {
		if p.Id != idPlayer {
			modifiedPlayer := *p.Player
			modifiedPlayer.Id = p.Id
			modifiedPlayer.Body.Position.Y -= 0.3
			playersCopy = append(playersCopy, modifiedPlayer)
		}
	}

	return playersCopy
}

func ConvertToPlayer(data string) *typegame.Player {
	var player typegame.Player
	err := json.Unmarshal([]byte(data), &player)
	if err != nil {
		log.Printf("No se pudo hacer la conversion: %s\nError: %v", data, err)
		return nil
	}
	return &player
}
