package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/marktsarkov/test/service"
)

func NewRouter(app *fiber.App, service service.Iservice) {
	app.Get("/counter/:bannerID", saveClick(service))
	app.Post("/stats/:bannerID", getStats(service))
}
