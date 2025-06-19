package ws

import (
	"github.com/DuvanRozoParra/servercbtic/internal/network/ws/handlers"
	"github.com/DuvanRozoParra/servercbtic/internal/network/ws/middleware"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func RouterWebsocket(app *fiber.App, jg *typegame.JobGame) {
	api := app.Group("/api")

	v1 := api.Group("/v1")
	v1.Use("/ws", middleware.ConnectWebsocket)
	v1.Get("/ws/:id", websocket.New(func(c *websocket.Conn) {
		handlers.Conn(c, jg)
	}))
}
