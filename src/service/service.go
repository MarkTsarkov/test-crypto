package service

import (
	"context"
	"github.com/google/uuid"
	"github.com/marktsarkov/test/errs"
	"github.com/marktsarkov/test/model"
	"github.com/marktsarkov/test/repo"
	"github.com/marktsarkov/test/txManager"
)

type service struct {
	repo repo.Irepo
	tx   txManager.TxManager
}

func NewService(repo repo.Irepo, tx txManager.TxManager) Iservice {
	return &service{
		repo: repo,
		tx:   tx,
	}
}

func (s *service) CreateWithdrawal(ctx context.Context, withdrawal *model.Withdrawal) (*model.Withdrawal, []byte, error) {
	var result *model.Withdrawal
	var oldResponse []byte
	err := s.tx.WithTx(ctx, func(ctx context.Context) (err error) {

		//lock idempotencyKey
		err = s.repo.LockIdempotency(ctx, withdrawal.IdempotencyKey, withdrawal.UserID)
		if err != nil {
			return err
		}
		//checkIdempotency and payload
		oldResponse, err = s.repo.CheckIdempotency(ctx, withdrawal)
		if err != nil {
			return err
		}
		if oldResponse != nil {
			return nil
		}
		//checkBalance
		userBalance, err := s.repo.CheckBalance(ctx, withdrawal)
		if err != nil {
			return err
		}
		networkFee := 1 //захардкодил значение, чтобы не писать логику проверки комиссии сети
		operationNeeded := withdrawal.Amount + networkFee
		if operationNeeded > userBalance {
			return errs.ErrPureBalance
		}
		//createOrder
		result, err = s.repo.CreateWithdrawal(ctx, withdrawal)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return result, oldResponse, nil
}

func (s *service) GetWithdrawals(ctx context.Context, userID uuid.UUID) ([]model.Withdrawal, error) {
	result, err := s.repo.GetWithdrawals(ctx, userID)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) ConfirmWithdrawal(ctx context.Context, operationID uuid.UUID) (*model.Withdrawal, error) {
	result, err := s.repo.ConfirmWithdrawal(ctx, operationID)
	//TODO: create row in ledger
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *service) SaveResponse(ctx context.Context, response []byte, withdrawal *model.Withdrawal) error {
	err := s.repo.SaveResponse(ctx, response, withdrawal)
	if err != nil {
		return err
	}
}
