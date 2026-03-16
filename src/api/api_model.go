package api

import (
	"time"

	"github.com/marktsarkov/test/model"
)

// ErrorResponse описывает тело ошибочного HTTP-ответа.
type ErrorResponse struct {
	Message string `json:"message"`
}

type WithdrawalRequest struct {
	UserID      string `json:"user_id"      validate:"required,uuid4"     example:"12345678-1234-4123-b234-123456789012"`
	Amount      int    `json:"amount"       validate:"required,gt=0"       example:"10"`
	Currency    string `json:"currency"     validate:"required,oneof=USDT" example:"USDT"`
	Destination string `json:"destination"  validate:"required"             example:"TJRyWwFtzThYgBstgex9NL4iuMMryZQkwy"`
}

type WithdrawalResponse struct {
	WithdrawalID   string    `json:"withdrawal_id"`
	UserID         string    `json:"user_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Amount         int       `json:"amount,omitzero"`
	Status         string    `json:"status,omitzero"`
	CreatedAt      time.Time `json:"created_at,omitzero"`
}

func withdrawalToResponse(response model.Withdrawal) WithdrawalResponse {
	return WithdrawalResponse{
		WithdrawalID:   response.OperationID.String(),
		UserID:         response.UserID.String(),
		IdempotencyKey: response.IdempotencyKey.String(),
		Amount:         response.Amount,
		Status:         response.Status,
		CreatedAt:      response.CreatedAt,
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
