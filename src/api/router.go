package api

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/marktsarkov/test/middleware"
	"github.com/marktsarkov/test/service"
)

func NewRouter(app *fiber.App, service service.Iservice, validator *validator.Validate) {
	api := app.Group("v1")
	api.Use(middleware.AuthMiddleware)
	api.Post("/withdrawals", createWithdrawal(service, validator))
	api.Get("/withdrawals/:id", getWithdrawal(service))
	api.Post("/withdrawals/:id/confirm", confirmWithdrawal(service))
}
