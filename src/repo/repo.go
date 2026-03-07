package repo

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marktsarkov/test/errs"
	"github.com/marktsarkov/test/logger"
	"github.com/marktsarkov/test/model"
	"github.com/marktsarkov/test/txManager"
	"hash/fnv"
)

type repo struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) Irepo {
	return &repo{db: db}
}

func (r *repo) LockIdempotency(ctx context.Context, key uuid.UUID, userID uuid.UUID) error {
	db, err := getDB(ctx)
	if err != nil {
		return err
	}

	lockID := hashKey(key.String(), userID.String())

	_, err = db.Exec(ctx,
		`SELECT pg_advisory_xact_lock($1)`,
		lockID,
	)
	if err != nil {
		logger.Fail("error: ", err)
		return err
	}
	return nil
}

func (r *repo) CheckIdempotency(ctx context.Context, withdrawal *model.Withdrawal) ([]byte, error) {
	db, err := getDB(ctx)
	if err != nil {
		return nil, err
	}

	var status string
	var storedHash string
	var code int
	var response []byte

	key := withdrawal.IdempotencyKey.String()
	userID := withdrawal.UserID.String()
	hash := withdrawal.HashedBody

	err = db.QueryRow(ctx,
		`SELECT status, request_hash, response, status_code 
			 FROM idempotency_keys
			 WHERE key=$1 AND user_id=$2
			 FOR UPDATE`,
		withdrawal.IdempotencyKey.String(), withdrawal.UserID.String(),
	).Scan(&status, &storedHash, &response, &code)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		logger.Fail("error: ", err)
		return nil, err
	}

	if storedHash != "" {
		if storedHash == hash {
			return response, nil
		} else if storedHash != hash {
			return nil, errs.ErrUnprocessableEntity
		}
	}
	_, err = db.Exec(ctx,
		`INSERT INTO idempotency_keys
			 (key,user_id,request_hash,status)
			 VALUES ($1,$2,$3,'pending')
			 ON CONFLICT DO NOTHING`,
		key, userID, hash)
	if err != nil {
		logger.Fail("error: ", err)
		return nil, err
	}
	return nil, nil
}

func (r *repo) CheckBalance(ctx context.Context, withdrawal *model.Withdrawal) (int, error) {
	db, err := getDB(ctx)
	if err != nil {
		return nil, err
	}
	var userBalance int
	err = db.QueryRow(ctx, `SELECT amount 
		FROM users_balances
		WHERE user_id=$1 AND currency=$2`,
		withdrawal.UserID, withdrawal.Currency).Scan(&userBalance)
	if err != nil {
		logger.Fail("error: ", err)
		return 0, err
	}
	return userBalance, nil
}

func (r *repo) CreateWithdrawal(ctx context.Context, w *model.Withdrawal) (*model.Withdrawal, error) {
	var result *model.Withdrawal
	var operationID uuid.UUID

	db, err := getDB(ctx)
	if err != nil {
		return nil, err
	}

	err = db.QueryRow(ctx,
		`INSERT INTO withdrawals (user_id, amount, currency, destination, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING operation_id, user_id, amount, currency, destination, idempotency_key`,
		w.UserID, w.Amount, w.Currency, w.Destination, w.IdempotencyKey,
	).Scan(&operationID, &result.UserID, &result.Amount, &result.Currency, &result.Destination, &result.IdempotencyKey)
	if err != nil {
		return nil, err
	}

	result.OperationID = operationID
	return result, nil
}

func (r *repo) SaveResponse(ctx context.Context, response []byte, withdrawal *model.Withdrawal) error {
	_, err := r.db.Exec(ctx,
		`UPDATE idempotency_keys 
			SET response=($1)
			WHERE key=($2) AND user_id=($3)`,
		response, withdrawal.IdempotencyKey, withdrawal.UserID)
	if err != nil {
		return err
	}
	return nil
}

func (r *repo) GetWithdrawals(ctx context.Context, id uuid.UUID) ([]model.Withdrawal, error) {
	rows, err := r.db.Query(ctx,
		`SELECT operation_id, user_id, amount, currency, destination, idempotency_key
		 FROM withdrawals
		 WHERE user_id = $1`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.Withdrawal
	for rows.Next() {
		var w model.Withdrawal
		var operationID uuid.UUID
		if err := rows.Scan(&operationID, &w.UserID, &w.Amount, &w.Currency, &w.Destination, &w.IdempotencyKey); err != nil {
			return nil, err
		}
		w.OperationID = operationID
		results = append(results, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (r *repo) ConfirmWithdrawal(ctx context.Context, operationID uuid.UUID) (*model.Withdrawal, error) {
	var result *model.Withdrawal
	var opID uuid.UUID

	err := r.db.QueryRow(ctx,
		`UPDATE withdrawals
		 SET status = 'complete'
		 WHERE operation_id = $1 AND status = 'pending'
		 RETURNING operation_id, status`,
		operationID,
	).Scan(&opID, &result.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}

	result.OperationID = opID
	return result, nil
}

func getDB(ctx context.Context) (DBTX, error) {
	tx, ok := ctx.Value(txManager.TxKey).(pgx.Tx)
	if !ok {
		return nil, errors.New("transaction manager not found in context")
	}
	return tx, nil
}

func hashKey(key string, userID string) int64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	h.Write([]byte(userID))
	return int64(h.Sum64())
}
