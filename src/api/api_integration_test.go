package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/marktsarkov/test/api"
	"github.com/marktsarkov/test/repo"
	"github.com/marktsarkov/test/service"
)

// TestCreateWithdrawal_Success — успешный вывод при достаточном балансе.
func TestCreateWithdrawal_Success(t *testing.T) {
	app, db := setupApp(t)
	userID := uuid.New()
	t.Cleanup(func() { cleanupUser(t, db, userID) })
	insertBalance(t, db, userID, 1000)

	resp, err := app.Test(makeRequest(userID, uuid.New().String()), 10000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["withdrawal_id"] == "" {
		t.Fatal("withdrawal_id is empty")
	}
	if result["user_id"] != userID.String() {
		t.Fatalf("user_id mismatch: want %s, got %s", userID, result["user_id"])
	}
}

// TestCreateWithdrawal_InsufficientBalance — отказ при нехватке баланса (amount=10, fee=1, balance=5).
func TestCreateWithdrawal_InsufficientBalance(t *testing.T) {
	app, db := setupApp(t)
	userID := uuid.New()
	t.Cleanup(func() { cleanupUser(t, db, userID) })
	insertBalance(t, db, userID, 5)

	resp, err := app.Test(makeRequest(userID, uuid.New().String()), 10000)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, b)
	}
}

// TestCreateWithdrawal_Idempotency — повторный запрос с тем же ключом возвращает тот же ответ без дубля в БД.
func TestCreateWithdrawal_Idempotency(t *testing.T) {
	app, db := setupApp(t)
	userID := uuid.New()
	t.Cleanup(func() { cleanupUser(t, db, userID) })
	insertBalance(t, db, userID, 1000)

	key := uuid.New().String()

	resp1, err := app.Test(makeRequest(userID, key), 10000)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp1.Body)
		t.Fatalf("first request: expected 200, got %d: %s", resp1.StatusCode, b)
	}
	var r1 map[string]string
	json.NewDecoder(resp1.Body).Decode(&r1)

	resp2, err := app.Test(makeRequest(userID, key), 10000)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("second request: expected 200, got %d: %s", resp2.StatusCode, b)
	}
	var r2 map[string]string
	json.NewDecoder(resp2.Body).Decode(&r2)

	if r1["withdrawal_id"] != r2["withdrawal_id"] {
		t.Fatalf("idempotency broken: %s != %s", r1["withdrawal_id"], r2["withdrawal_id"])
	}

	var count int
	db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM withdrawals WHERE user_id = $1`, userID,
	).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 withdrawal in DB, got %d", count)
	}
}

// TestCreateWithdrawal_Concurrent — 10 параллельных запросов с одним ключом идемпотентности
// должны создать ровно одну запись в БД и вернуть одинаковый withdrawal_id.
func TestCreateWithdrawal_Concurrent(t *testing.T) {
	app, db := setupApp(t)
	userID := uuid.New()
	t.Cleanup(func() { cleanupUser(t, db, userID) })
	insertBalance(t, db, userID, 1000)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go app.Listener(ln)                  //nolint:errcheck
	t.Cleanup(func() { app.Shutdown() }) //nolint:errcheck
	time.Sleep(50 * time.Millisecond)

	addr := "http://" + ln.Addr().String()
	key := uuid.New().String()
	body, _ := json.Marshal(map[string]any{
		"user_id":     userID.String(),
		"amount":      10,
		"currency":    "USDT",
		"destination": "TRxTestAddress123",
	})

	const n = 10
	type result struct {
		status int
		id     string
	}
	results := make([]result, n)
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start

			req, _ := http.NewRequest(http.MethodPost, addr+"/v1/withdrawals", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer testtoken")
			req.Header.Set("Idempotency-Key", key)

			resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
			if err != nil {
				t.Errorf("goroutine %d: %v", i, err)
				return
			}
			defer resp.Body.Close()

			var r map[string]string
			json.NewDecoder(resp.Body).Decode(&r)
			results[i] = result{status: resp.StatusCode, id: r["withdrawal_id"]}
		}(i)
	}

	close(start) // все горутины стартуют одновременно
	wg.Wait()

	var expectedID string
	for i, r := range results {
		if r.status != http.StatusOK {
			t.Errorf("goroutine %d: expected 200, got %d", i, r.status)
			continue
		}
		if expectedID == "" {
			expectedID = r.id
		} else if r.id != expectedID {
			t.Errorf("goroutine %d: withdrawal_id mismatch: want %s, got %s", i, expectedID, r.id)
		}
	}

	var count int
	db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM withdrawals WHERE user_id = $1`, userID,
	).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 withdrawal in DB, got %d (race condition!)", count)
	}
}

var testDSN string

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:15-alpine3.17",
		tcpostgres.WithDatabase("operations"),
		tcpostgres.WithUsername("user"),
		tcpostgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}
	defer pgContainer.Terminate(ctx)

	testDSN, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("get connection string: %v", err)
	}

	sqlDB, err := sql.Open("pgx", testDSN)
	if err != nil {
		log.Fatalf("open sql db for migrations: %v", err)
	}
	if err := goose.Up(sqlDB, "../../pkg/postgres/migrations"); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	sqlDB.Close()

	os.Exit(m.Run())
}

func setupApp(t *testing.T) (*fiber.App, *pgxpool.Pool) {
	t.Helper()
	os.Setenv("USER_TOKEN", "testtoken")

	cfg, err := pgxpool.ParseConfig(testDSN)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.MaxConns = 30
	db, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	t.Cleanup(db.Close)

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	api.NewRouter(app, service.NewService(repo.NewRepo(db)), db, validator.New())
	return app, db
}

func insertBalance(t *testing.T, db *pgxpool.Pool, userID uuid.UUID, amount int) {
	t.Helper()
	_, err := db.Exec(context.Background(),
		`INSERT INTO users_balances (user_id, currency, amount) VALUES ($1, 'USDT', $2)
		 ON CONFLICT (user_id, currency) DO UPDATE SET amount = $2`,
		userID, amount,
	)
	if err != nil {
		t.Fatalf("insert balance: %v", err)
	}
}

func cleanupUser(t *testing.T, db *pgxpool.Pool, userID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	db.Exec(ctx, `DELETE FROM withdrawals WHERE user_id = $1`, userID)
	db.Exec(ctx, `DELETE FROM idempotency_keys WHERE user_id = $1`, userID.String())
	db.Exec(ctx, `DELETE FROM users_balances WHERE user_id = $1`, userID)
}

func makeRequest(userID uuid.UUID, idempotencyKey string) *http.Request {
	body, _ := json.Marshal(map[string]any{
		"user_id":     userID.String(),
		"amount":      10,
		"currency":    "USDT",
		"destination": "TRxTestAddress123",
	})
	req, _ := http.NewRequest(http.MethodPost, "/v1/withdrawals", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer testtoken")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	return req
}
