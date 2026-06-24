package handlers

import (
	"time"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/DuvanRozoParra/servercbtic/pkg"
	"github.com/gofiber/contrib/websocket"
	"github.com/rs/zerolog/log"
)

func Conn(c *websocket.Conn, jg *typegame.JobGame) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("Conn panic recuperado")
		}
	}()

	id := c.Params("id")

	msgAddUser, player, err := pkg.AddUser(jg, c, id)
	if err != nil {
		if pkg.KickPlayer(jg, id) {
			log.Info().Str("id", id).Msg("reemplazando conexión existente")
			msgAddUser, player, err = pkg.AddUser(jg, c, id)
		}
	}
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("rechazando conexión")
		if closeErr := c.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Str("id", id).Msg("error al cerrar socket tras rechazo")
		}
		return
	}

	defer func() {
		msgDelete := pkg.DesconnectUser(jg, c, id)
		select {
		case jg.Queue <- msgDelete:
		default:
			log.Warn().Str("id", id).Msg("queue full al encolar RemovePlayer, descartando")
		}
		log.Info().Str("id", id).Msg("jugador eliminado")
	}()

	pkg.StartWriter(player)

	select {
	case jg.Queue <- *msgAddUser:
	default:
		log.Warn().Str("id", id).Msg("queue full al encolar AddPlayer, descartando")
	}

	log.Info().Str("id", id).Msg("jugador agregado")

	_ = c.SetReadDeadline(time.Now().Add(config.ReadTimeout))
	c.SetPongHandler(func(appData string) error {
		_ = c.SetReadDeadline(time.Now().Add(config.ReadTimeout))
		return nil
	})
	c.SetReadLimit(1 << 20)

	var (
		msg         []byte
		readErr     error
		msgCount    int
		windowStart = time.Now()
	)

	for {
		if _, msg, readErr = c.ReadMessage(); readErr != nil {
			log.Debug().Err(readErr).Str("id", id).Msg("read loop terminado")
			break
		}

		_ = c.SetReadDeadline(time.Now().Add(config.ReadTimeout))

		now := time.Now()
		if now.Sub(windowStart) >= time.Second {
			windowStart = now
			msgCount = 0
		}
		msgCount++
		if msgCount > config.IngressRatePerSec {
			log.Warn().Str("id", id).Int("rate", msgCount).Msg("ingress rate limit, dropping")
			continue
		}

		message, err := pkg.ByteToMessageObject(msg)
		if err != nil {
			log.Warn().Err(err).Str("id", id).Msg("error al deserializar el mensaje")
			continue
		}

		select {
		case jg.Queue <- message:
		default:
			log.Warn().Str("id", id).Msg("queue full, dropping message")
		}
	}
}
