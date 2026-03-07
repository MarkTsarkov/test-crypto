package api

import (
	"github.com/marktsarkov/test/model"
)

type WithdrawalRequest struct {
	UserID      string `json:"user_id" validate:"required,uuid4"`
	Amount      int    `json:"amount" validate:"required,gt=0"`
	Currency    string `json:"currency" validate:"required,oneof=USDT"`
	Destination string `json:"destination" validate:"required"`
}

type WithdrawalResponse struct {
	WithdrawalID   string `json:"withdrawal_id"`
	UserID         string `json:"user_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func withdrawalToResponse(response model.Withdrawal) WithdrawalResponse {
	return WithdrawalResponse{
		WithdrawalID:   response.OperationID.String(),
		UserID:         response.UserID.String(),
		IdempotencyKey: response.IdempotencyKey.String(),
	}
}

type ConfirmWithdrawalResponse struct {
	OperationID string `json:"operation_id"`
	Status      string `json:"status"`
}

func confirmWithdrawalToResponse(response model.Withdrawal) ConfirmWithdrawalResponse {
	return ConfirmWithdrawalResponse{
		OperationID: response.OperationID.String(),
		Status:      response.Status,
	}
}
