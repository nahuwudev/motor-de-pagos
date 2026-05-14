package idempotency

type Status string

const (
	Pending   Status = "Pending"
	Failed    Status = "Failed"
	Completed Status = "Completed"
)
