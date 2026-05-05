package payments

import "context"

type PaymentService struct {
	repo PaymentRepository // Interfaz limpia, sin rastros de pgx
}

func NewPaymentService(repo PaymentRepository) *PaymentService {
	return &PaymentService{repo: repo}
}

// El servicio es estúpido y feliz, no sabe qué base de datos hay atrás
func (s *PaymentService) ProcessTransfer(ctx context.Context, req TransferParams) error {
	// Validaciones extra de negocio previas...

	// Delegar ejecución atómica a la capa de datos
	return s.repo.ExecuteTransferTx(ctx, req)
}
