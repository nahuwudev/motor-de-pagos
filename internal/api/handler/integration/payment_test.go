package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"main/internal/db"
	"main/internal/payments"
	"sync"

	_ "github.com/lib/pq"

	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestCreateTransfer_HappyPath(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)

	fromAccount, err := queries.CreateAccount(ctx, db.CreateAccountParams{
		OwnerID:  uuid.New(),
		Balance:  1000,
		Currency: "ARS",
	})
	if err != nil {
		t.Fatalf("error creando cuenta origen: %v", err)
	}

	toAccount, err := queries.CreateAccount(ctx, db.CreateAccountParams{
		OwnerID:  uuid.New(),
		Balance:  0,
		Currency: "ARS",
	})
	if err != nil {
		t.Fatalf("error creando cuenta destino: %v", err)
	}

	body := payments.TransferParams{
		FromAccountID: fromAccount.ID,
		ToAccountID:   toAccount.ID,
		Amount:        500,
		Currency:      "ARS",
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("error marshalling body: %v", err)
	}

	req, err := http.NewRequest("POST", testServer.URL+"/api/v1/payments", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("error creando request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", uuid.New().String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("error mandando request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("esperaba 201, got %d", resp.StatusCode)
	}
}

func TestCreateTransfer_DoubleClick(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)

	fromAccount, err := queries.CreateAccount(ctx, db.CreateAccountParams{
		OwnerID:  uuid.New(),
		Balance:  1000,
		Currency: "ARS",
	})
	if err != nil {
		t.Fatalf("error creando cuenta origen: %v", err)
	}

	toAccount, err := queries.CreateAccount(ctx, db.CreateAccountParams{
		OwnerID:  uuid.New(),
		Balance:  0,
		Currency: string(payments.CurrencyARS),
	})
	if err != nil {
		t.Fatalf("error creando cuenta destino: %v", err)
	}

	body := payments.TransferParams{
		FromAccountID: fromAccount.ID,
		ToAccountID:   toAccount.ID,
		Amount:        500,
		Currency:      payments.CurrencyARS,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("error marshalling body: %v", err)
	}

	idemKey := uuid.New().String()
	results := make([]int, 2) // capturarr los status codes

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			req, err := http.NewRequest("POST", testServer.URL+"/api/v1/payments", bytes.NewReader(bodyBytes))
			if err != nil {
				t.Errorf("error creando request: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", idemKey)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("error mandando request: %v", err)
				return
			}
			defer resp.Body.Close()
			results[index] = resp.StatusCode
		}(i)
	}

	wg.Wait()

	codes := map[int]int{}
	for _, code := range results {
		codes[code]++
	}

	if codes[http.StatusCreated] == 0 {
		t.Errorf("esperaba al menos un 201, got: %v", results)
	}

	if codes[http.StatusInternalServerError] > 0 {
		t.Errorf("hubo errores internos: %v", results)
	}

	txs, err := queries.ListAccountTransactions(ctx, db.ListAccountTransactionsParams{
		FromAccountID: fromAccount.ID,
		Limit:         10,
		Offset:        0,
	})
	if err != nil {
		t.Fatalf("error listando transacciones: %v", err)
	}

	if len(txs) != 1 {
		t.Errorf("esperaba 1 transacción, got %d", len(txs))
	}
}

func TestCreateTransfer_InsufficientFunds(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)

	fromAccount, err := queries.CreateAccount(ctx, db.CreateAccountParams{
		OwnerID:  uuid.New(),
		Balance:  0,
		Currency: "ARS",
	})

	if err != nil {
		log.Fatalf("Error creating fromAccount %v", err)
	}

	toAccount, err := queries.CreateAccount(ctx, db.CreateAccountParams{
		OwnerID:  uuid.New(),
		Balance:  0,
		Currency: "ARS",
	})

	if err != nil {
		log.Fatalf("Error creating toAccount %v", err)
	}

	body := payments.TransferParams{
		FromAccountID: fromAccount.ID,
		ToAccountID:   toAccount.ID,
		Amount:        500,
		Currency:      "ARS",
	}

	data, err := json.Marshal(body)

	if err != nil {
		log.Fatalf("Error formatting json %v", err)
	}

	idemKey := uuid.New().String()
	url := testServer.URL + "/api/v1/payments"

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		log.Fatalf("Error sending request %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-key", idemKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Error obtaining response %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)

		t.Errorf(
			"esperaba %d, obtuve %d. body: %s",
			http.StatusBadRequest,
			resp.StatusCode,
			string(body),
		)
	}

}
