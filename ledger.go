package main

import (
	"errors"
	"sync"
	"time"
)

const SYSTEM_ACCOUNT_ID = "system"

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

type JournalEntry struct {
	TxID           string    `json:"tx_id"`
	AccountID      string    `json:"account_id"`
	CounterpartyID string    `json:"counterparty_id"`
	Timestamp      time.Time `json:"timestamp"`
	Debit          float64   `json:"debit,omitempty"`
	Credit         float64   `json:"credit,omitempty"`
}

type WithdrawRequest struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

type DepositRequest struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

type Ledger struct {
	mu           sync.RWMutex
	balances     map[string]float64
	transactions []Transaction
	seenTx       map[string]bool
}

func NewLedger() *Ledger {
	return &Ledger{
		balances:     make(map[string]float64),
		transactions: []Transaction{},
		seenTx:       make(map[string]bool),
	}
}

func (l *Ledger) Deposit(dr DepositRequest) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.seenTx[dr.ID] {
		return false, nil
	}
	if dr.Amount <= 0 {
		return false, errors.New("amount must be > 0")
	}

	l.balances[dr.AccountID] += dr.Amount
	l.balances[SYSTEM_ACCOUNT_ID] -= dr.Amount
	l.transactions = append(l.transactions, Transaction{
		ID:        dr.ID,
		Timestamp: time.Now(),
		Entries: []Entry{
			{AccountID: dr.AccountID, Credit: dr.Amount},
			{AccountID: SYSTEM_ACCOUNT_ID, Debit: dr.Amount},
		},
	})
	l.seenTx[dr.ID] = true
	return true, nil
}

func (l *Ledger) Withdraw(wr WithdrawRequest) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.seenTx[wr.ID] {
		return false, nil
	}
	if wr.Amount <= 0 {
		return false, errors.New("amount must be > 0")
	}
	if l.balances[wr.AccountID] < wr.Amount {
		return false, errors.New("insufficient funds")
	}

	l.transactions = append(l.transactions, Transaction{
		ID:        wr.ID,
		Timestamp: time.Now(),
		Entries: []Entry{
			{AccountID: wr.AccountID, Debit: wr.Amount},
			{AccountID: SYSTEM_ACCOUNT_ID, Credit: wr.Amount},
		},
	})
	l.balances[wr.AccountID] -= wr.Amount
	l.balances[SYSTEM_ACCOUNT_ID] += wr.Amount
	l.seenTx[wr.ID] = true
	return true, nil
}

func (l *Ledger) Transfer(t Transfer) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.seenTx[t.ID] {
		return false, nil // already processed
	}
	if t.Amount <= 0 {
		return false, errors.New("amount must be > 0")
	}
	if t.DebitAccountID == t.CreditAccountID {
		return false, errors.New("debit and credit account must differ")
	}
	if l.balances[t.DebitAccountID] < t.Amount {
		return false, errors.New("insufficient funds")
	}

	l.balances[t.DebitAccountID] -= t.Amount
	l.balances[t.CreditAccountID] += t.Amount

	l.transactions = append(l.transactions, Transaction{
		ID:        t.ID,
		Timestamp: time.Now(),
		Entries: []Entry{
			{AccountID: t.DebitAccountID, Debit: t.Amount},
			{AccountID: t.CreditAccountID, Credit: t.Amount},
		},
	})
	l.seenTx[t.ID] = true
	return true, nil
}

func (l *Ledger) Balances(strong bool) map[string]float64 {
	if strong {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}

	copy := make(map[string]float64)
	for k, v := range l.balances {
		copy[k] = v
	}
	return copy
}

func (l *Ledger) Transactions(strong bool) []Transaction {
	if strong {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}

	copy := make([]Transaction, len(l.transactions))
	copy = append(copy[:0], l.transactions...)
	return copy
}

func (l *Ledger) Journal(accountID string, strong bool) []JournalEntry {
	if strong {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}

	var rows []JournalEntry
	for _, tx := range l.transactions {
		if len(tx.Entries) != 2 {
			continue
		}

		var entry, counter Entry
		if tx.Entries[0].AccountID == accountID {
			entry = tx.Entries[0]
			counter = tx.Entries[1]
		} else if tx.Entries[1].AccountID == accountID {
			entry = tx.Entries[1]
			counter = tx.Entries[0]
		} else {
			continue
		}

		rows = append(rows, JournalEntry{
			TxID:           tx.ID,
			AccountID:      accountID,
			CounterpartyID: counter.AccountID,
			Timestamp:      tx.Timestamp,
			Debit:          entry.Debit,
			Credit:         entry.Credit,
		})
	}
	return rows
}
