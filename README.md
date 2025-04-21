# Tiny Ledger

A minimal in-memory ledger exploration written in Go, inspired by TigerBeetle's approach to high-integrity accounting systems.

## Overview

This is a toy ledger implementation that supports:

- Transfers between accounts
- Deposits and withdrawals
- Per-account journal views
- Simple idempotency (via transaction IDs)

All data is stored in-memory. There's no persistence, and restarting the app resets all state.

## Design Notes

- All balance-affecting operations are modeled as double-entry `Transaction`s.
- Transactions are assumed to have exactly two `Entry` objects (e.g. a debit and a credit). In a future version this could be easily extendable to multi-acc tx.
- A global mutex protects all ledger state (balances, transactions, seenTx). To improve this naive design we could go for per-account locking, or implement a event loop style command queue ("single threaded event loop") for serializable processing
- Reads are assumed strong reads (i.e. RLock is used to read the latest) but in reality I think we could skip the lock or make strongly consistent reads optional via a param
- Idempotency is enforced using a `seenTx` map — duplicate requests with the same ID will return `302 Found`.
- the API expects tx ids to be passed in, in reality we would generate a time based unique idempotency id in this place
- for simplicity the transactions get committed right away but in a real world scenario we'd use 2-phase commits 

## To Run
You can pull a new devbox env locally that should have all deps, alternatively install go 1.21 locally
```shell
devbox shell
go get
air # this should hot-reload main and run our api, you can also go run main.go
```

## API

### POST `/withdraw`
```
{
  "id": "txn-id",
  "account_id": "acct-A",
  "amount": 100.0,
  "timestamp": "2025-01-01T00:00:00Z"
}
```

### POST `/deposit`
```json
{
  "id": "txn-id",
  "account_id": "acct-A",
  "amount": 100.0,
  "timestamp": "2025-01-01T00:00:00Z"
}

```

### POST `/transfer`
transfers the amount between accs (if balance allows)

```json
{
  "id": "txn-id",
  "debit_account_id": "acct-A",
  "credit_account_id": "acct-B",
  "amount": 10.0,
  "timestamp": "2025-01-01T00:00:00Z"
}
```

### GET `/accounts/balances`
Returns all accounts & balances, just to make it easy to peek inside the system status. In a real world scenario we'd most likely use some form of snapshotting and derive balances from an append-only journal. This would allow us to support reconstruction, historical queries, and improve concurrency by avoiding direct balance mutation. 

### GET `/transactions`
Returns the full transaction log as-is, in the order they were processed. This endpoint is useful for inspecting raw activity across all accounts. In a more complete system, this would likely be append-only, persisted, and potentially queryable by account, date, or type.

### GET `/accounts/:id/journal`
Returns all journal entries affecting the given account. For simplicity this is reconstructed from the transaction slice.
