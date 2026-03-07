package middleware

import (
	"crypto/subtle"
	"github.com/gofiber/fiber/v2"
	"os"
	"strings"
)

func AuthMiddleware(c *fiber.Ctx) error {
	auth := c.Get("Authorization")

	if auth == "" {
		return c.Status(fiber.StatusUnauthorized).SendString("missing token")
	}

	const prefix = "Bearer "

	if !strings.HasPrefix(auth, prefix) {
		return c.Status(fiber.StatusUnauthorized).SendString("invalid auth header")
	}

	token := strings.TrimPrefix(auth, prefix)
	userToken := os.Getenv("USER_TOKEN") //чтобы не ходить за настоящим токеном
	if subtle.ConstantTimeCompare([]byte(token), []byte(userToken)) == 0 {
		return c.Status(fiber.StatusUnauthorized).SendString("invalid token")
	}

	return c.Next()
}
