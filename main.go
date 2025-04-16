package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Entry struct {
	AccountID string  `json:"account_id"`
	Debit     float64 `json:"debit,omitempty"`
	Credit    float64 `json:"credit,omitempty"`
}

type Transaction struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Entries   []Entry   `json:"entries"`
}

type Transfer struct {
	ID              string    `json:"id"`
	DebitAccountID  string    `json:"debit_account_id"`
	CreditAccountID string    `json:"credit_account_id"`
	Amount          float64   `json:"amount"`
	Timestamp       time.Time `json:"timestamp"`
}

type WithdrawRequest struct {
	ID        string    `json:"id"` // unique txn ID
	AccountID string    `json:"account_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	mu           sync.RWMutex
	transactions []Transaction
	// processed    = make(map[string]bool) // we should track idempotent ids and respond Found for processed tx
	balances = make(map[string]float64)
)

func getTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	json.NewEncoder(w).Encode(transactions)
}

func processTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var t Transfer
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if t.Amount <= 0 || t.DebitAccountID == t.CreditAccountID {
		http.Error(w, "invalid transfer", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if balances[t.DebitAccountID] < t.Amount {
		http.Error(w, "insufficient funds", http.StatusConflict)
		return
	}

	balances[t.DebitAccountID] -= t.Amount
	balances[t.CreditAccountID] += t.Amount

	transactions = append(transactions, Transaction{
		ID:        t.ID,
		Timestamp: time.Now(),
		Entries: []Entry{
			{AccountID: t.DebitAccountID, Debit: t.Amount},
			{AccountID: t.CreditAccountID, Credit: t.Amount},
		},
	})

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(t)
}

func withdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var wr WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&wr); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if wr.Amount <= 0 {
		http.Error(w, "amount must be greater than zero", http.StatusBadRequest)
		return
	}

}

func main() {
	http.HandleFunc("/transfer", processTransfer)
	http.HandleFunc("/transactions", getTransactions)
	http.HandleFunc("/withdraw", withdraw)
	http.ListenAndServe(":8080", nil)
}
