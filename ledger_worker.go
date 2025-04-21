package main

import (
	"context"
)

type CommandType int

const (
	CmdDeposit CommandType = iota
	CmdWithdraw
	CmdTransfer
)

type Command struct {
	Type    CommandType
	Payload any
	Resp    chan error
}

var (
	cmdQueue = make(chan Command, 1024)
	worker   *Worker
)

func startLedgerWorker(ledger *Ledger) {
	worker = &Worker{
		Name: "ledger",
		Handler: func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case cmd := <-cmdQueue:
					var err error
					switch cmd.Type {
					case CmdDeposit:
						_, err = ledger.Deposit(cmd.Payload.(DepositRequest))
					case CmdWithdraw:
						_, err = ledger.Withdraw(cmd.Payload.(WithdrawRequest))
					case CmdTransfer:
						_, err = ledger.Transfer(cmd.Payload.(Transfer))
					}
					cmd.Resp <- err
				}
			}
		},
		OnPanic: func(r any) {
			// log / alert / retry
		},
	}
	worker.Start()
}
