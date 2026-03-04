package main

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/marktsarkov/test/api"
	"github.com/marktsarkov/test/repo"
	"github.com/marktsarkov/test/service"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := fiber.New()
	DSN := "postgres://user:password@postgres:5432/clicks?sslmode=disable"
	pool, err := pgxpool.Connect(ctx, DSN)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	r := repo.NewClickRepo(pool)
	s := service.NewService(r)
	s.ParallelSender(ctx)
	api.NewRouter(app, s)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			cancel()
			s.Close(ctx)
		default:
		}
	}()

	log.Println("Listening on :8080...")
	log.Fatal(app.Listen(":8080"))
}
