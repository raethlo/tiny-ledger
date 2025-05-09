package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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

var (
	mu           sync.RWMutex
	transactions []Transaction
	balances     = make(map[string]float64)
	seenTx       = make(map[string]bool)
)

func getTransactions(c *gin.Context) {
	log.Println("GET /transactions")
	mu.RLock()
	defer mu.RUnlock()
	c.JSON(http.StatusOK, transactions)
}

func getBalances(c *gin.Context) {
	log.Println("GET /balances")
	mu.RLock()
	defer mu.RUnlock()
	c.JSON(http.StatusOK, balances)
}

func processTransaction(id string, transactions []Transaction, entries []Entry) []Transaction {
	now := time.Now() // in reality we would compare client clock and server clock to detect drift?
	tx := Transaction{
		ID:        id,
		Timestamp: now,
		Entries:   entries,
	}
	var updated = append(transactions, tx)
	seenTx[id] = true
	return updated
}

func processTransfer(c *gin.Context) {
	var t Transfer
	if err := c.ShouldBindJSON(&t); err != nil {
		log.Println("POST /transfer - invalid json")
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	log.Printf("POST /transfer - ID=%s Debit=%s Credit=%s Amount=%.2f\n",
		t.ID, t.DebitAccountID, t.CreditAccountID, t.Amount)

	if seenTx[t.ID] {
		c.JSON(http.StatusFound, gin.H{"message": "transaction already processed"})
		return
	}

	if t.Amount <= 0 || t.DebitAccountID == t.CreditAccountID || t.CreditAccountID == SYSTEM_ACCOUNT_ID || t.DebitAccountID == SYSTEM_ACCOUNT_ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transfer"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if balances[t.DebitAccountID] < t.Amount {
		c.JSON(http.StatusConflict, gin.H{"error": "insufficient funds"})
		return
	}

	balances[t.DebitAccountID] -= t.Amount
	balances[t.CreditAccountID] += t.Amount

	transactions = processTransaction(t.ID, transactions, []Entry{
		{AccountID: t.DebitAccountID, Debit: t.Amount},
		{AccountID: t.CreditAccountID, Credit: t.Amount},
	})

	c.JSON(http.StatusCreated, t)
}

func withdraw(c *gin.Context) {
	var wr WithdrawRequest
	if err := c.ShouldBindJSON(&wr); err != nil {
		log.Println("POST /withdraw - invalid json")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	log.Printf("POST /withdraw - ID=%s AccountID=%s Amount=%.2f\n",
		wr.ID, wr.AccountID, wr.Amount)

	if seenTx[wr.ID] {
		c.JSON(http.StatusFound, gin.H{"message": "transaction already processed"})
		return
	}

	if wr.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than zero"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if balances[wr.AccountID] < wr.Amount {
		c.JSON(http.StatusConflict, gin.H{"error": "insufficient funds"})
		return
	}

	balances[wr.AccountID] -= wr.Amount

	transactions = processTransaction(wr.ID, transactions, []Entry{
		{AccountID: wr.AccountID, Debit: wr.Amount},
		{AccountID: SYSTEM_ACCOUNT_ID, Credit: wr.Amount},
	})

	c.Status(http.StatusCreated)
}

func deposit(c *gin.Context) {
	var dr DepositRequest

	if err := c.ShouldBindJSON(&dr); err != nil {
		log.Println("POST /withdraw - invalid json")
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	log.Printf("POST /deposit - ID=%s AccountID=%s Amount=%.2f\n", dr.ID, dr.AccountID, dr.Amount)

	if seenTx[dr.ID] {
		c.JSON(http.StatusFound, gin.H{"message": "transaction already processed"})
		return
	}

	if dr.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than zero"})
		return
	}

	mu.Lock()
	defer mu.Unlock()

	balances[dr.AccountID] += dr.Amount

	transactions = processTransaction(dr.ID, transactions, []Entry{
		{AccountID: dr.AccountID, Credit: dr.Amount},
		{AccountID: SYSTEM_ACCOUNT_ID, Debit: dr.Amount},
	})

	c.Status(http.StatusCreated)
}

func getAccountJournal(c *gin.Context) {
	accountID := c.Param("id")

	mu.RLock()
	defer mu.RUnlock()

	var rows []JournalEntry
	for _, tx := range transactions {
		if len(tx.Entries) != 2 {
			panic("multi acc tx not supported yet")
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
			AccountID:      entry.AccountID,
			CounterpartyID: counter.AccountID,
			Timestamp:      tx.Timestamp,
			Debit:          entry.Debit,
			Credit:         entry.Credit,
		})
	}

	if len(rows) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no entries for account"})
		return
	}

	c.JSON(http.StatusOK, rows)
}

func main() {
	r := gin.Default()
	balances["A"] = 100
	balances["B"] = 100
	balances[SYSTEM_ACCOUNT_ID] = -200

	r.GET("/transactions", getTransactions)
	r.GET("/balances", getBalances)

	r.GET("/accounts/:id/journal", getAccountJournal)

	r.POST("/transfer", processTransfer)
	r.POST("/withdraw", withdraw)
	r.POST("/deposit", deposit)

	r.Run(":8080")
}
