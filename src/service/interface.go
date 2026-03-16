package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/marktsarkov/test/model"
)

type Iservice interface {
	CreateWithdrawal(ctx context.Context, withdrawal *model.Withdrawal) (*model.Withdrawal, []byte, error)
	GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]model.Withdrawal, error)
	ConfirmWithdrawal(ctx context.Context, operationID uuid.UUID) (*model.Withdrawal, error)
	SaveResponse(ctx context.Context, response []byte, withdrawal *model.Withdrawal) error
}
