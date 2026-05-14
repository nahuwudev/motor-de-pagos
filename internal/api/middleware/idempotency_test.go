package middleware_test

import (
	"context"
	"main/internal/api/middleware"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockStore struct {
	acquireResult  bool
	getResponseVal []byte
}

func (m *mockStore) AcquireLock(ctx context.Context, lockKey, ownerID string, ttl time.Duration) (bool, error) {
	return m.acquireResult, nil
}
func (m *mockStore) SetPending(ctx context.Context, dataKey string, ttl time.Duration) error {
	return nil
}
func (m *mockStore) GetResponse(ctx context.Context, dataKey string) ([]byte, error) {
	return m.getResponseVal, nil
}
func (m *mockStore) SaveResponse(ctx context.Context, lockKey, dataKey, ownerID string, payload []byte, ttl time.Duration) error {
	return nil
}
func (m *mockStore) DeleteLock(ctx context.Context, lockKey, ownerID string) error { return nil }
func (m *mockStore) ForceDelete(ctx context.Context, key string) error             { return nil }

func TestIdempotencyMiddleware_NoKey(t *testing.T) {
	store := &mockStore{}
	handler := middleware.IdempotencyMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// no debería ejecutarse en este test
	}))

	req := httptest.NewRequest("POST", "/", http.NoBody)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("esperaba 400, got %d", rec.Code)
	}
}

func TestIdempotencyMiddleware_Case1(t *testing.T) {
	idempotencyKey := "x"

	payload := []byte(`{"status_code":200,"body":{"id":"123"},"Header":{"Content-Type":["application/json"]}}`)
	store := &mockStore{
		acquireResult:  false,
		getResponseVal: payload,
	}

	handler := middleware.IdempotencyMiddleware(store)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		}),
	)

	req := httptest.NewRequest("POST", "/", http.NoBody)
	req.Header.Set("Idempotency-Key", idempotencyKey)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("esperaba 200, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("esperaba Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
	}

	if rec.Body.String() != `{"id":"123"}` {
		t.Errorf("esperaba body %s, got %s", `{"id":"123"}`, rec.Body.String())
	}
}

func TestIdempotencyMiddleware_Case2(t *testing.T) {
	idempotencyKey := "x"

	store := &mockStore{
		acquireResult: true,
	}

	handler := middleware.IdempotencyMiddleware(store)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"123"}`))
		}),
	)

	req := httptest.NewRequest("POST", "/", http.NoBody)
	req.Header.Set("Idempotency-Key", idempotencyKey)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("esperaba 201, got %d", rec.Code)
	}

	if rec.Body.String() != `{"id":"123"}` {
		t.Errorf("esperaba body %s, got %s", `{"id":"123"}`, rec.Body.String())
	}
}
