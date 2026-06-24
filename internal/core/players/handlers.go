package players

import (
	"encoding/json"

	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/rs/zerolog/log"
)

func GetAllPlayerId(jg *typegame.JobGame, idPlayer string) []typegame.AddPlayerMsg {
	playersIds := make([]typegame.AddPlayerMsg, 0, len(jg.Players))

	for _, p := range jg.Players {
		playersIds = append(playersIds, typegame.AddPlayerMsg{
			Id:    p.Id,
			Color: p.Color,
		})
	}

	return playersIds
}

func ConvertToPlayer(data string) *typegame.Player {
	var player typegame.Player
	err := json.Unmarshal([]byte(data), &player)
	if err != nil {
		log.Warn().Err(err).Msg("No se pudo hacer la conversion")
		return nil
	}
	return &player
}
