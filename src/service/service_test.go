//go:build test

package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/marktsarkov/test/errs"
	"github.com/marktsarkov/test/model"
	repomocks "github.com/marktsarkov/test/repo/mocks"
	"github.com/marktsarkov/test/service"
	txmocks "github.com/marktsarkov/test/txManager/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	errLock    = errors.New("advisory lock failed")
	errBalance = errors.New("no rows in result set")
)

// setupTxPassThrough настраивает мок TxManager так, чтобы он выполнял fn напрямую.
func setupTxPassThrough(tx *txmocks.MockTxManager) {
	tx.EXPECT().WithTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
}

func newWithdrawal() *model.Withdrawal {
	return &model.Withdrawal{
		UserID:         uuid.New(),
		Amount:         100,
		Currency:       "USDT",
		Destination:    "addr123",
		IdempotencyKey: uuid.New(),
		HashedBody:     "abc123hash",
	}
}

// --- TestCreateWithdrawal: успех + ошибка + edge-кейсы ---
func TestCreateWithdrawal(t *testing.T) {
	cached := []byte(`{"withdrawal_id":"old","user_id":"u","idempotency_key":"k"}`)

	tests := []struct {
		name        string
		amount      int
		setup       func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal)
		wantResult  bool
		wantOldResp []byte
		wantErrIs   error
	}{
		{
			name:   "success",
			amount: 100,
			setup: func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal) {
				setupTxPassThrough(tx)
				expected := &model.Withdrawal{OperationID: uuid.New(), UserID: w.UserID}
				r.EXPECT().LockIdempotency(mock.Anything, w.IdempotencyKey, w.UserID).Return(nil)
				r.EXPECT().CheckIdempotency(mock.Anything, w).Return(nil, nil)
				r.EXPECT().CheckBalance(mock.Anything, w).Return(1000, nil)
				r.EXPECT().CreateWithdrawal(mock.Anything, w).Return(expected, nil)
			},
			wantResult: true,
		},
		{
			// amount=1000, fee=1 → operationNeeded=1001 > 100(balance) → ErrPureBalance
			name:   "insufficient balance",
			amount: 1000,
			setup: func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal) {
				setupTxPassThrough(tx)
				r.EXPECT().LockIdempotency(mock.Anything, w.IdempotencyKey, w.UserID).Return(nil)
				r.EXPECT().CheckIdempotency(mock.Anything, w).Return(nil, nil)
				r.EXPECT().CheckBalance(mock.Anything, w).Return(100, nil)
			},
			wantErrIs: errs.ErrPureBalance,
		},
		{
			// Повторный запрос с тем же ключом и телом возвращает кэшированный ответ.
			name:   "idempotency: cached response returned",
			amount: 100,
			setup: func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal) {
				setupTxPassThrough(tx)
				r.EXPECT().LockIdempotency(mock.Anything, w.IdempotencyKey, w.UserID).Return(nil)
				r.EXPECT().CheckIdempotency(mock.Anything, w).Return(cached, nil)
			},
			wantOldResp: cached,
		},
		{
			// Тот же idempotency key, но тело запроса отличается → ErrUnprocessableEntity.
			name:   "edge: conflicting body for same idempotency key",
			amount: 100,
			setup: func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal) {
				setupTxPassThrough(tx)
				r.EXPECT().LockIdempotency(mock.Anything, w.IdempotencyKey, w.UserID).Return(nil)
				r.EXPECT().CheckIdempotency(mock.Anything, w).Return(nil, errs.ErrUnprocessableEntity)
			},
			wantErrIs: errs.ErrUnprocessableEntity,
		},
		{
			// Ошибка при захвате advisory lock — дальнейшие методы не вызываются.
			name:   "edge: advisory lock error",
			amount: 100,
			setup: func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal) {
				setupTxPassThrough(tx)
				r.EXPECT().LockIdempotency(mock.Anything, w.IdempotencyKey, w.UserID).Return(errLock)
			},
			wantErrIs: errLock,
		},
		{
			// Ошибка при чтении баланса (нет записи пользователя в таблице).
			name:   "edge: balance check error",
			amount: 100,
			setup: func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal) {
				setupTxPassThrough(tx)
				r.EXPECT().LockIdempotency(mock.Anything, w.IdempotencyKey, w.UserID).Return(nil)
				r.EXPECT().CheckIdempotency(mock.Anything, w).Return(nil, nil)
				r.EXPECT().CheckBalance(mock.Anything, w).Return(0, errBalance)
			},
			wantErrIs: errBalance,
		},
		{
			// amount=99, fee=1 → operationNeeded=100, balance=100; 100 > 100 == false → успех.
			name:   "edge: boundary amount+fee == balance (success)",
			amount: 99,
			setup: func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal) {
				setupTxPassThrough(tx)
				expected := &model.Withdrawal{OperationID: uuid.New(), UserID: w.UserID, Amount: 99}
				r.EXPECT().LockIdempotency(mock.Anything, w.IdempotencyKey, w.UserID).Return(nil)
				r.EXPECT().CheckIdempotency(mock.Anything, w).Return(nil, nil)
				r.EXPECT().CheckBalance(mock.Anything, w).Return(100, nil)
				r.EXPECT().CreateWithdrawal(mock.Anything, w).Return(expected, nil)
			},
			wantResult: true,
		},
		{
			// amount=100, fee=1 → operationNeeded=101, balance=100; 101 > 100 == true → ошибка.
			name:   "edge: boundary amount+fee > balance by 1 (fail)",
			amount: 100,
			setup: func(r *repomocks.MockIrepo, tx *txmocks.MockTxManager, w *model.Withdrawal) {
				setupTxPassThrough(tx)
				r.EXPECT().LockIdempotency(mock.Anything, w.IdempotencyKey, w.UserID).Return(nil)
				r.EXPECT().CheckIdempotency(mock.Anything, w).Return(nil, nil)
				r.EXPECT().CheckBalance(mock.Anything, w).Return(100, nil)
			},
			wantErrIs: errs.ErrPureBalance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWithdrawal()
			w.Amount = tt.amount

			r := repomocks.NewMockIrepo(t)
			tx := txmocks.NewMockTxManager(t)
			tt.setup(r, tx, w)

			svc := service.NewService(r, tx)
			result, oldResp, err := svc.CreateWithdrawal(context.Background(), w)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Nil(t, result)
				assert.Nil(t, oldResp)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOldResp, oldResp)
			if tt.wantResult {
				assert.NotNil(t, result)
			}
		})
	}
}

// --- TestConfirmWithdrawal ---

func TestConfirmWithdrawal(t *testing.T) {
	opID := uuid.New()
	confirmed := &model.Withdrawal{OperationID: opID, Status: "complete"}

	tests := []struct {
		name      string
		setup     func(r *repomocks.MockIrepo)
		wantRes   *model.Withdrawal
		wantErrIs error
	}{
		{
			name: "success",
			setup: func(r *repomocks.MockIrepo) {
				r.EXPECT().ConfirmWithdrawal(mock.Anything, opID).Return(confirmed, nil)
				r.EXPECT().SaveLedger(mock.Anything, opID).Return(nil)
			},
			wantRes: confirmed,
		},
		{
			// Операция не найдена или уже подтверждена → ErrNotFound.
			name: "not found",
			setup: func(r *repomocks.MockIrepo) {
				r.EXPECT().ConfirmWithdrawal(mock.Anything, opID).Return(nil, errs.ErrNotFound)
			},
			wantErrIs: errs.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repomocks.NewMockIrepo(t)
			tx := txmocks.NewMockTxManager(t)
			tt.setup(r)

			svc := service.NewService(r, tx)
			result, err := svc.ConfirmWithdrawal(context.Background(), opID)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRes, result)
		})
	}
}

// --- TestGetWithdrawals ---

func TestGetWithdrawals(t *testing.T) {
	userID := uuid.New()

	tests := []struct {
		name      string
		setup     func(r *repomocks.MockIrepo)
		wantLen   int
		wantErrIs error
	}{
		{
			name: "returns list",
			setup: func(r *repomocks.MockIrepo) {
				r.EXPECT().GetWithdrawals(mock.Anything, userID).Return([]model.Withdrawal{
					{UserID: userID, Amount: 10},
					{UserID: userID, Amount: 20},
				}, nil)
			},
			wantLen: 2,
		},
		{
			// Пользователь без выводов — пустой слайс, не ошибка.
			name: "edge: empty result for user without withdrawals",
			setup: func(r *repomocks.MockIrepo) {
				r.EXPECT().GetWithdrawals(mock.Anything, userID).Return([]model.Withdrawal{}, nil)
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := repomocks.NewMockIrepo(t)
			tx := txmocks.NewMockTxManager(t)
			tt.setup(r)

			svc := service.NewService(r, tx)
			result, err := svc.GetWithdrawals(context.Background(), userID)

			if tt.wantErrIs != nil {
				require.ErrorIs(t, err, tt.wantErrIs)
				return
			}
			require.NoError(t, err)
			assert.Len(t, result, tt.wantLen)
		})
	}
}
