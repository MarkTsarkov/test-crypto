package repo

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/marktsarkov/test/model"
)

type Irepo interface {
	CreateWithdrawal(ctx context.Context, withdrawal *model.Withdrawal) (*model.Withdrawal, error)
	GetWithdrawals(ctx context.Context, id uuid.UUID) ([]model.Withdrawal, error)
	ConfirmWithdrawal(ctx context.Context, operationID uuid.UUID) (*model.Withdrawal, error)

	LockIdempotency(ctx context.Context, key uuid.UUID, userID uuid.UUID) error
	CheckIdempotency(ctx context.Context, withdrawal *model.Withdrawal) ([]byte, error)
	CheckBalance(ctx context.Context, withdrawal *model.Withdrawal) (int, error)

	SaveResponse(ctx context.Context, response []byte, withdrawal *model.Withdrawal) error
}

type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}
