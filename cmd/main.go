package main

import (
	"log"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	"github.com/DuvanRozoParra/servercbtic/internal/network/queue"
	"github.com/DuvanRozoParra/servercbtic/internal/network/ws"
	"github.com/gofiber/fiber/v2"
)

func main() {
	// 1. Crear la instancia principal del juego
	jobsGame := queue.New(config.QueueBufferSize)

	// 2. Configurar servidor web
	app := fiber.New()

	// 3. Configurar WebSockets
	ws.RouterWebsocket(app, jobsGame)

	// 4. Iniciar workers
	queue.StartWorkers(config.QueueWorkerSize, jobsGame)

	// 5. Iniciar el servidor
	log.Fatal(app.Listen(config.Address))
}
