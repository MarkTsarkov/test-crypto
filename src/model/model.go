package model

import (
	"github.com/google/uuid"
)

type Withdrawal struct {
	UserID         uuid.UUID
	Amount         int
	Currency       string
	Destination    string
	OperationID    uuid.UUID
	Status         string
	IdempotencyKey uuid.UUID
	HashedBody     string
}
