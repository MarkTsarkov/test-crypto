//go:build test

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/marktsarkov/test/errs"
	"github.com/marktsarkov/test/model"
	"github.com/marktsarkov/test/repo"
	"github.com/marktsarkov/test/service"
	"github.com/marktsarkov/test/txManager"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testUserID соответствует записи из миграции (баланс: 1000 USDT).
var testUserID = uuid.MustParse("12345678-1234-4123-b234-123456789012")

var testConnStr string

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer pgContainer.Terminate(ctx)

	testConnStr, err = pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get connection string: %v\n", err)
		os.Exit(1)
	}

	if err := runMigrations(testConnStr); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func runMigrations(connStr string) error {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "../../pkg/postgres/migrations")
}

func newTestService(t *testing.T) service.Iservice {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	r := repo.NewRepo(pool)
	tx := txManager.NewTxManager(pool)
	return service.NewService(r, tx)
}

func newTestWithdrawal() *model.Withdrawal {
	return &model.Withdrawal{
		UserID:         testUserID,
		Amount:         10,
		Currency:       "USDT",
		Destination:    "addr_test",
		IdempotencyKey: uuid.New(),
		HashedBody:     uuid.New().String(),
	}
}

// --- TestIntegration_CreateWithdrawal: 2 основных теста + edge-кейс ---

func TestIntegration_CreateWithdrawal(t *testing.T) {
	tests := []struct {
		name        string
		makeW       func() *model.Withdrawal
		wantErrIs   error
		checkResult func(t *testing.T, result *model.Withdrawal, oldResp []byte)
	}{
		{
			name:  "success",
			makeW: newTestWithdrawal,
			checkResult: func(t *testing.T, result *model.Withdrawal, oldResp []byte) {
				require.NotNil(t, result)
				assert.Nil(t, oldResp)
				assert.Equal(t, testUserID, result.UserID)
				assert.NotEqual(t, uuid.Nil, result.OperationID)
			},
		},
		{
			// 2000 + 1(fee) = 2001 > 1000(balance) → ErrPureBalance.
			name: "insufficient balance",
			makeW: func() *model.Withdrawal {
				w := newTestWithdrawal()
				w.Amount = 2000
				return w
			},
			wantErrIs: errs.ErrPureBalance,
		},
		{
			// Тот же idempotency key и тело, но другой хэш — конфликт.
			name: "edge: conflicting body for same idempotency key",
			makeW: func() *model.Withdrawal {
				// Первый запрос будет выполнен внутри теста, здесь только второй.
				return nil // сигнал: тест управляет двумя вызовами сам
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t)

			// Специальная логика для теста с конфликтом тела.
			if tt.name == "edge: conflicting body for same idempotency key" {
				key := uuid.New()
				userID := testUserID

				w1 := newTestWithdrawal()
				w1.IdempotencyKey = key
				w1.HashedBody = "hash-body-1"
				w1.UserID = userID

				_, _, err := svc.CreateWithdrawal(context.Background(), w1)
				require.NoError(t, err)

				w2 := newTestWithdrawal()
				w2.IdempotencyKey = key
				w2.HashedBody = "hash-body-2"
				w2.UserID = userID

				_, _, err = svc.CreateWithdrawal(context.Background(), w2)
				require.ErrorIs(t, err, errs.ErrUnprocessableEntity)
				return
			}

			w := tt.makeW()
			result, oldResp, err := svc.CreateWithdrawal(context.Background(), w)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Nil(t, result)
				assert.Nil(t, oldResp)
				return
			}
			require.NoError(t, err)
			tt.checkResult(t, result, oldResp)
		})
	}
}

// --- TestIntegration_Idempotency ---

func TestIntegration_Idempotency(t *testing.T) {
	svc := newTestService(t)
	w := newTestWithdrawal()

	// Первый вызов — создаёт вывод.
	result1, oldResp1, err := svc.CreateWithdrawal(context.Background(), w)
	require.NoError(t, err)
	require.NotNil(t, result1)
	assert.Nil(t, oldResp1)

	// Сохраняем ответ в кэш идемпотентности (как это делает API-хэндлер).
	respBytes := []byte(`{"withdrawal_id":"` + result1.OperationID.String() + `"}`)
	require.NoError(t, svc.SaveResponse(context.Background(), respBytes, result1))

	// Второй вызов с тем же ключом и телом — возвращает кэш.
	result2, oldResp2, err := svc.CreateWithdrawal(context.Background(), w)
	require.NoError(t, err)
	assert.Nil(t, result2)
	require.NotNil(t, oldResp2)
	assert.Equal(t, respBytes, oldResp2)
}

// --- TestIntegration_ConcurrentCreate: параллельные create на один баланс ---
// Несколько горутин одновременно создают выводы для одного пользователя
// с разными idempotency key. Проверяем отсутствие дедлоков/гонок: все должны завершиться успешно.

func TestIntegration_ConcurrentCreate(t *testing.T) {
	svc := newTestService(t)

	const goroutines = 5
	var wg sync.WaitGroup
	var successCount atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &model.Withdrawal{
				UserID:         testUserID,
				Amount:         10, // 10 + 1(fee) = 11, well within 1000 balance
				Currency:       "USDT",
				Destination:    "addr_concurrent",
				IdempotencyKey: uuid.New(),
				HashedBody:     uuid.New().String(),
			}
			result, _, err := svc.CreateWithdrawal(context.Background(), w)
			if err == nil && result != nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(goroutines), successCount.Load())
}
