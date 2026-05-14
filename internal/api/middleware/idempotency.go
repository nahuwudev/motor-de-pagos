package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"main/internal/idempotency"
	"maps"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	header     http.Header
}

type cachedResponse struct {
	StatusCode int             `json:"status_code"`
	Body       json.RawMessage `json:"body"`
	Header     http.Header
}

func (rec *responseRecorder) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
	maps.Copy(rec.ResponseWriter.Header(), rec.header)
	rec.ResponseWriter.WriteHeader(statusCode)
}

func (rec *responseRecorder) Write(b []byte) (int, error) {
	rec.body.Write(b)
	return rec.ResponseWriter.Write(b)
}

func (rec *responseRecorder) Header() http.Header {
	return rec.header
}

func IdempotencyMiddleware(redisStore idempotency.IdempotencyRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method := r.Method

			if method == "" || method == "HEAD" {
				next.ServeHTTP(w, r)
				return
			}

			idemKey := r.Header.Get("Idempotency-key")
			if idemKey == "" {
				http.Error(w, `{"error":"Idempotency-Key header is required"}`, http.StatusBadRequest)
				return
			}

			if len(idemKey) > 255 {
				http.Error(w, `{"error":"Idempotency-Key too long"}`, http.StatusBadRequest)
				return
			}

			lockTTL := 60 * time.Second
			lockKey := "idem:lock:" + idemKey // aca guardo UUID
			dataTTL := 24 * time.Hour
			dataKey := "idem:data:" + idemKey // aca guardo PENDING, FAILED, o el JSON
			ctx := r.Context()
			ownerID := uuid.NewString()

			acquired, err := redisStore.AcquireLock(ctx, lockKey, ownerID, lockTTL)
			if err != nil {
				log.Printf("Redis error acquiring lock: %v", err)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				return
			}

			// =========================
			// CASE 1: ALREADY EXISTS
			// =========================

			if !acquired {
				data, err := redisStore.GetResponse(ctx, dataKey)
				if err != nil {
					if errors.Is(err, idempotency.ErrKeyNotFound) {
						http.Error(w, `{"error":" missing idempotency key"}`, http.StatusConflict)
						return
					}
					log.Printf("Redis error getting response: %v", err)
					http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
					return
				}

				if string(data) == string(idempotency.Pending) {
					http.Error(w, `{"error": "Request already in progress"}`, http.StatusConflict)
					return
				}

				if string(data) == string(idempotency.Failed) {
					_ = redisStore.ForceDelete(ctx, lockKey)
					http.Error(w, `{"error": "Previous attempt failed, please retry"}`, http.StatusServiceUnavailable)
					return
				}

				var cached cachedResponse
				if err := json.Unmarshal(data, &cached); err != nil {
					log.Printf("Corrupted cache for key %s: %v", lockKey, err)
					http.Error(w, `{"error":"cache corrupted"}`, http.StatusInternalServerError)
					return
				}

				maps.Copy(w.Header(), cached.Header)
				w.WriteHeader(cached.StatusCode)
				_, _ = w.Write(cached.Body) // golangci xD
				return
			}

			// =========================
			// CASE 2: FIRST EXECUTION
			// =========================
			if err := redisStore.SetPending(ctx, dataKey, dataTTL); err != nil {
				log.Printf("Redis error set pending key: %v", err)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				return
			}

			panicked := true
			defer func() {
				if panicked {
					cleanupCtx := context.Background()
					_ = redisStore.SaveResponse(cleanupCtx, lockKey, dataKey, ownerID, []byte(idempotency.Failed), dataTTL)
					_ = redisStore.DeleteLock(cleanupCtx, lockKey, ownerID)
				}
			}()

			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           bytes.NewBuffer(nil),
				header:         make(http.Header),
			}

			next.ServeHTTP(rec, r)

			// build cached response

			if rec.statusCode >= 200 && rec.statusCode < 300 {
				cached := cachedResponse{
					StatusCode: rec.statusCode,
					Body:       rec.body.Bytes(),
					Header:     rec.Header(),
				}

				payload, err := json.Marshal(cached)
				if err != nil {
					log.Printf("Error marshalling cached response: %v", err)
					return
				}

				if err := redisStore.SaveResponse(ctx, lockKey, dataKey, ownerID, payload, dataTTL); err != nil {
					log.Printf("Error saving response in Redis: %v", err)
					return
				}

				panicked = false
			} else {
				_ = redisStore.DeleteLock(ctx, lockKey, ownerID)
				panicked = false
			}

		})
	}
}
