package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	"github.com/DuvanRozoParra/servercbtic/internal/network/queue"
	"github.com/DuvanRozoParra/servercbtic/internal/network/ws"
	"github.com/DuvanRozoParra/servercbtic/pkg"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	level, err := zerolog.ParseLevel(config.LogLevel)
	if err != nil {
		log.Warn().Str("got", config.LogLevel).Msg("CBTIC_LOG_LEVEL inválido, usando info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	jobsGame := queue.New(config.QueueBufferSize)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	ws.RouterWebsocket(app, jobsGame)
	queue.StartWorkers(config.QueueWorkerSize, jobsGame)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info().Msg("shutting down: signal received")

		wire, err := pkg.BuildWire("server_shutdown", "server", config.ServerShutdown)
		if err == nil {
			pkg.SendAllMsg(jobsGame, websocket.TextMessage, wire)
			log.Info().Msg("evento server_shutdown enviado")
		}

		time.Sleep(500 * time.Millisecond)

		jobsGame.Mu.RLock()
		for _, p := range jobsGame.Players {
			if p != nil && p.Conn != nil {
				_ = p.Conn.SetReadDeadline(time.Now())
			}
		}
		jobsGame.Mu.RUnlock()
		log.Info().Msg("conexiones señaladas para cierre")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("graceful shutdown failed")
		}
	}()

	log.Info().Str("addr", config.Address).Msg("server listening")
	if err := app.Listen(config.Address); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
