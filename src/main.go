package main

import (
	"context"
	"github.com/marktsarkov/test/txManager"
	"log"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/marktsarkov/test/api"
	"github.com/marktsarkov/test/repo"
	"github.com/marktsarkov/test/service"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file, using environment variables")
	}

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN is required")
	}

	ctx := context.Background()

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("failed to connect db: %v", err)
	}
	defer db.Close()

	tx := txManager.NewTxManager(db)
	r := repo.NewRepo(db)
	svc := service.NewService(r, tx)
	v := validator.New()

	app := fiber.New()
	api.NewRouter(app, svc, db, v)

	log.Println("Listening on :8080...")
	log.Fatal(app.Listen(":8080"))
}
