package handler

import (
	"encoding/json"
	"errors"
	"log"
	"main/internal/payments"
	"net/http"

	"github.com/google/uuid"
)

type PaymentHandler struct {
	service *payments.PaymentService
}

func NewPaymentHandler(service *payments.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {

	idempotencyKey := r.Header.Get("idempotency-key")

	if idempotencyKey == "" {
		http.Error(w, "missing idempotency key", http.StatusBadRequest)
		return
	}

	var transferParams payments.TransferParams

	err := json.NewDecoder(r.Body).Decode(&transferParams)
	if err != nil {
		http.Error(w, "failed to decode params", http.StatusBadRequest)
		return
	}

	if transferParams.FromAccountID == uuid.Nil {
		http.Error(w, "from_account_id is required", http.StatusBadRequest)
		return
	}

	if transferParams.ToAccountID == uuid.Nil {
		http.Error(w, "to_account_id is required", http.StatusBadRequest)
		return
	}

	if transferParams.Currency == "" {
		http.Error(w, "currency is required", http.StatusBadRequest)
		return
	}

	result, err := h.service.ProcessTransfer(r.Context(), transferParams, idempotencyKey)

	if err != nil {
		switch {
		case errors.Is(err, payments.ErrInvalidAmount):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return

		case errors.Is(err, payments.ErrAccountNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
			return

		case errors.Is(err, payments.ErrCurrencyMismatch):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return

		case errors.Is(err, payments.ErrSameAccountTransfer):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return

		case errors.Is(err, payments.ErrInsufficientFunds):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return

		case errors.Is(err, payments.ErrCurrencyRequired):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return

		case errors.Is(err, payments.ErrInvalidCurrency):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return

		case errors.Is(err, payments.ErrIdempotencyConflict):
			http.Error(w, err.Error(), http.StatusConflict)
			return

		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
