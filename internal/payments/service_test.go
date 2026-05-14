package payments

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type mockPaymentRepo struct {
	mockResult TransferResult
	mockError  error
	called     bool
}

func (m *mockPaymentRepo) ExecuteTransferTx(ctx context.Context, arg TransferParams) (TransferResult, error) {
	m.called = true
	return m.mockResult, m.mockError
}

func TestProcessTransfer_Success(t *testing.T) {
	mockTxID := uuid.New()

	repoTrucho := &mockPaymentRepo{
		mockResult: TransferResult{TransactionID: mockTxID, Status: string(TransactionStatusCompleted)},
		mockError:  nil,
	}

	service := NewPaymentService(repoTrucho)

	req := TransferParams{
		FromAccountID: uuid.New(),
		ToAccountID:   uuid.New(),
		Amount:        1000,
		Currency:      "ARS",
	}

	result, err := service.ProcessTransfer(context.Background(), req, "idemp-key-123")

	if err != nil {
		t.Errorf("Se esperaba que no hubiera error, pero dio: %v", err)
	}

	if result.TransactionID != mockTxID {
		t.Errorf("Se esperaba el TransactionID %v, pero se recibió %v", mockTxID, result.TransactionID)
	}

	if result.Status != string(TransactionStatusCompleted) {
		t.Errorf("Se esperaba el status %s, pero se recibió %s", TransactionStatusCompleted, result.Status)
	}
}

func TestProcessTransfer_RepoError(t *testing.T) {
	repoFake := &mockPaymentRepo{
		mockError: ErrInsufficientFunds,
	}

	service := NewPaymentService(repoFake)

	req := TransferParams{
		FromAccountID: uuid.New(),
		ToAccountID:   uuid.New(),
		Amount:        100,
		Currency:      CurrencyARS,
	}

	_, err := service.ProcessTransfer(
		context.Background(),
		req,
		"idempotency-key",
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf(
			"expected ErrInsufficientFunds, got %v",
			err,
		)
	}
}

func TestProcessTransfer_InvalidAmount(t *testing.T) {
	repoFake := &mockPaymentRepo{}

	if repoFake.called {
		t.Error("Repo should not be called for invalid requests")
	}

	service := NewPaymentService(repoFake)

	req := TransferParams{
		FromAccountID: uuid.New(),
		ToAccountID:   uuid.New(),
		Amount:        -500,
		Currency:      "ARS",
	}

	_, err := service.ProcessTransfer(context.Background(), req, "idemp-key-123")

	// Assert
	if err == nil {
		t.Error("Se esperaba un error por monto inválido, pero dio nil")
	}

	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("Se esperaba el error ErrInvalidAmount, pero dio: %v", err)
	}
}
