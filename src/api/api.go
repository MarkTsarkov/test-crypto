package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/marktsarkov/test/errs"
	"github.com/marktsarkov/test/logger"
	"github.com/marktsarkov/test/model"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marktsarkov/test/service"
)

func createWithdrawal(service service.Iservice, validator *validator.Validate) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()

		idempotencyKey := c.Get("Idempotency-Key")
		if idempotencyKey == "" {
			return fiber.NewError(fiber.StatusBadRequest, "Idempotency-Key is required")
		}
		key, err := uuid.Parse(idempotencyKey)
		if err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "Wrong Idempotency-Key")
		}

		body := c.Body()
		var req WithdrawalRequest
		err = json.Unmarshal(body, &req)
		if err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusBadRequest)
		}
		if err = validator.Struct(req); err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}

		userID, err := uuid.Parse(req.UserID)
		if err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusInternalServerError)
		}

		withdrawal := model.Withdrawal{
			UserID:         userID,
			Amount:         req.Amount,
			Currency:       req.Currency,
			Destination:    req.Destination,
			IdempotencyKey: key,
			HashedBody:     hashBody(body),
		}

		withdrawalResult, oldResponse, err := service.CreateWithdrawal(ctx, &withdrawal)
		if err != nil {
			logger.Fail("error:", err)
			if errors.Is(err, errs.ErrPureBalance) {
				return fiber.NewError(fiber.StatusConflict)
			}
			return fiber.NewError(fiber.StatusInternalServerError)
		}
		if oldResponse != nil {
			return c.Send(oldResponse)
		}
		resp := withdrawalToResponse(*withdrawalResult)

		respToSave, err := json.Marshal(resp)
		if err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusInternalServerError)
		}
		err = service.SaveResponse(ctx, respToSave, withdrawalResult)
		if err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusInternalServerError)
		}

		return c.JSON(resp)
	}
}

func confirmWithdrawal(service service.Iservice) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()

		operationID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusBadRequest, "invalid operation id")
		}

		result, err := service.ConfirmWithdrawal(ctx, operationID)
		if err != nil {
			if errors.Is(err, errs.ErrNotFound) {
				return fiber.NewError(fiber.StatusNotFound, "withdrawal not found")
			}
			if errors.Is(err, errs.ErrWrongStatus) {
				return fiber.NewError(fiber.StatusConflict, "withdrawal is not in pending status")
			}
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusInternalServerError)
		}

		resp := confirmWithdrawalToResponse(*result)
		return c.JSON(resp)
	}
}

func getWithdrawal(service service.Iservice) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()

		userID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusBadRequest)
		}

		withdrawals, err := service.GetWithdrawals(ctx, userID)
		if err != nil {
			logger.Fail("error:", err)
			return fiber.NewError(fiber.StatusInternalServerError)
		}

		resp := make([]WithdrawalResponse, 0, len(withdrawals))
		for _, w := range withdrawals {
			resp = append(resp, withdrawalToResponse(w))
		}
		return c.JSON(resp)
	}
}

func hashBody(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
