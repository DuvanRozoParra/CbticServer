package queue

import (
	"github.com/DuvanRozoParra/servercbtic/internal/config"
	"github.com/DuvanRozoParra/servercbtic/internal/core/players"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/DuvanRozoParra/servercbtic/pkg"
	"github.com/gofiber/contrib/websocket"
	"github.com/rs/zerolog/log"
)

func StartWorkers(workerSize int, jg *typegame.JobGame) {
	for i := range workerSize {
		go worker(i+1, jg)
	}
}

func worker(idWorker int, jg *typegame.JobGame) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Int("worker", idWorker).Interface("panic", r).Msg("worker goroutine panic (fatal)")
		}
	}()
	log.Info().Int("worker", idWorker).Msg("worker started")
	for msg := range jg.Queue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Int("worker", idWorker).Str("from", msg.From).Interface("panic", r).Msg("msg dropped after panic")
				}
			}()
			processWorker(jg, msg)
		}()
	}
}

func processWorker(jg *typegame.JobGame, msg typegame.MessageObject) {
	switch msg.Event {
	case config.AddPlayer:
		wire := func() []byte {
			jg.Mu.RLock()
			defer jg.Mu.RUnlock()

			dataStr, err := pkg.MarshalEventData(players.EventAddPlayer(jg, msg.From))
			if err != nil {
				log.Warn().Err(err).Str("from", msg.From).Msg("AddPlayer: marshal error, descartando")
				return nil
			}
			w, err := pkg.BuildWire(dataStr, msg.From, config.AddPlayer)
			if err != nil {
				log.Warn().Err(err).Str("from", msg.From).Msg("AddPlayer: wire build error, descartando")
				return nil
			}
			return w
		}()
		if wire != nil {
			pkg.SendAllMsg(jg, websocket.BinaryMessage, wire)
		}

	case config.MovePlayer:
		wire := func() []byte {
			jg.Mu.Lock()
			defer jg.Mu.Unlock()

			playerData := players.EventMovement(jg, msg.From, msg.Data)
			if playerData == nil {
				log.Warn().Str("from", msg.From).Msg("MovePlayer: payload inválido o jugador ausente, descartando")
				return nil
			}
			dataStr, err := pkg.MarshalEventData(playerData)
			if err != nil {
				log.Warn().Err(err).Str("from", msg.From).Msg("MovePlayer: marshal error, descartando")
				return nil
			}
			w, err := pkg.BuildWire(dataStr, msg.From, config.MovePlayer)
			if err != nil {
				log.Warn().Err(err).Str("from", msg.From).Msg("MovePlayer: wire build error, descartando")
				return nil
			}
			return w
		}()
		if wire != nil {
			pkg.SendAllExcept(jg, msg.From, websocket.TextMessage, wire)
		}

	case config.RayInteraction:
		wire := func() []byte {
			jg.Mu.RLock()
			defer jg.Mu.RUnlock()

			if _, ok := jg.Players[msg.From]; !ok {
				return nil
			}
			dataStr := players.EventRayInteraction(jg, msg.From, msg.Data)
			w, err := pkg.BuildWire(dataStr, msg.From, config.RayInteraction)
			if err != nil {
				log.Warn().Err(err).Str("from", msg.From).Msg("RayInteraction: wire build error, descartando")
				return nil
			}
			return w
		}()
		if wire != nil {
			pkg.SendAllMsg(jg, websocket.BinaryMessage, wire)
		}

	case config.ActionHandsPlayer:
		wire := func() []byte {
			jg.Mu.RLock()
			defer jg.Mu.RUnlock()

			if _, ok := jg.Players[msg.From]; !ok {
				return nil
			}
			dataStr := players.EventActionsHandsAnimation(jg, msg.From, msg.Data)
			w, err := pkg.BuildWire(dataStr, msg.From, config.ActionHandsPlayer)
			if err != nil {
				log.Warn().Err(err).Str("from", msg.From).Msg("ActionHandsPlayer: wire build error, descartando")
				return nil
			}
			return w
		}()
		if wire != nil {
			pkg.SendAllMsg(jg, websocket.BinaryMessage, wire)
		}

	case config.RemovePlayer:
		wire, err := pkg.BuildWire("", msg.From, config.RemovePlayer)
		if err != nil {
			log.Warn().Err(err).Str("from", msg.From).Msg("RemovePlayer: wire build error, descartando")
			return
		}
		pkg.SendAllMsg(jg, websocket.BinaryMessage, wire)

	default:
		log.Warn().Int("event", int(msg.Event)).Msg("evento desconocido")
	}
}
