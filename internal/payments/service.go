package payments

import "context"

type PaymentService struct {
	repo PaymentRepository
}

func NewPaymentService(repo PaymentRepository) *PaymentService {
	return &PaymentService{repo: repo}
}

func (c Currency) IsValid() bool {
	switch c {
	case CurrencyARS, CurrencyUSD, CurrencyEUR:
		return true
	default:
		return false
	}
}

func (s *PaymentService) ProcessTransfer(ctx context.Context, req TransferParams, idempotencyKey string) (TransferResult, error) {

	if req.Amount <= 0 {
		return TransferResult{}, ErrInvalidAmount
	}

	if req.Currency == "" {
		return TransferResult{}, ErrCurrencyRequired
	}

	if req.FromAccountID == req.ToAccountID {
		return TransferResult{}, ErrSameAccountTransfer
	}

	if !req.Currency.IsValid() {
		return TransferResult{}, ErrInvalidCurrency
	}

	// Delegar ejecución atómica a la capa de datos
	txRecord, err := s.repo.ExecuteTransferTx(ctx, req)
	if err != nil {
		return TransferResult{}, err
	}

	return txRecord, nil
}
