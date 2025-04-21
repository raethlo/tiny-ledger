package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var ledger *Ledger

func main() {
	r := gin.Default()
	ledger = NewLedger()

	// Init state
	ledger.Deposit(DepositRequest{
		ID:        "init-deposit-a",
		AccountID: "A",
		Amount:    100,
		Timestamp: time.Now(),
	})
	ledger.Deposit(DepositRequest{
		ID:        "init-deposit-b",
		AccountID: "B",
		Amount:    100,
		Timestamp: time.Now(),
	})

	// V1: direct ledger access with locking
	r.POST("/deposit", depositV1)
	r.POST("/withdraw", withdrawV1)
	r.POST("/transfer", transferV1)

	r.GET("/balances", getBalances)
	r.GET("/transactions", getTransactions)
	r.GET("/accounts/:id/journal", getJournal)

	// V2: queued processing
	r.POST("/v2/deposit", depositV2)
	r.POST("/v2/withdraw", withdrawV2)
	r.POST("/v2/transfer", transferV2)

	startLedgerWorker(ledger)

	r.Run(":8080")
}

// V1 handlers

func depositV1(c *gin.Context) {
	var dr DepositRequest
	if err := c.ShouldBindJSON(&dr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	created, err := ledger.Deposit(dr)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if !created {
		c.JSON(http.StatusFound, gin.H{"message": "transaction already processed"})
		return
	}

	c.JSON(http.StatusCreated, dr)
}

func withdrawV1(c *gin.Context) {
	var wr WithdrawRequest
	if err := c.ShouldBindJSON(&wr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	created, err := ledger.Withdraw(wr)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if !created {
		c.JSON(http.StatusFound, gin.H{"message": "transaction already processed"})
		return
	}

	c.JSON(http.StatusCreated, wr)
}

func transferV1(c *gin.Context) {
	var t Transfer
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	created, err := ledger.Transfer(t)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	if !created {
		c.JSON(http.StatusFound, gin.H{"message": "transaction already processed"})
		return
	}

	c.JSON(http.StatusCreated, t)
}

// Read endpoints (strong read enabled)

func getBalances(c *gin.Context) {
	c.JSON(http.StatusOK, ledger.Balances(true))
}

func getTransactions(c *gin.Context) {
	c.JSON(http.StatusOK, ledger.Transactions(true))
}

func getJournal(c *gin.Context) {
	accountID := c.Param("id")
	entries := ledger.Journal(accountID, true)
	if len(entries) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no entries"})
		return
	}
	c.JSON(http.StatusOK, entries)
}

func depositV2(c *gin.Context) {
	var dr DepositRequest
	if err := c.ShouldBindJSON(&dr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	resp := make(chan error)
	cmdQueue <- Command{Type: CmdDeposit, Payload: dr, Resp: resp}
	if err := <-resp; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusCreated)
}

func withdrawV2(c *gin.Context) {
	var wr WithdrawRequest
	if err := c.ShouldBindJSON(&wr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	resp := make(chan error)
	cmdQueue <- Command{Type: CmdWithdraw, Payload: wr, Resp: resp}
	if err := <-resp; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusCreated)
}

func transferV2(c *gin.Context) {
	var t Transfer
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	resp := make(chan error)
	cmdQueue <- Command{Type: CmdTransfer, Payload: t, Resp: resp}
	if err := <-resp; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}
