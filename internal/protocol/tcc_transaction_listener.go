package protocol

import "fmt"

type ReserveHandler func(e *Transaction) error
type ConfirmHandler func(e *Transaction) error
type CancelHandler func(e *Transaction) error

const (
	TCC_RESERVING uint32 = 1
	TCC_CONFIRMED uint32 = 2
	TCC_CANCELED  uint32 = 3

	TCC_FINISHING uint32 = 4
	TCC_FINISHED  uint32 = 5

	TCC_TRANSACTION_TIMEOUT uint32 = 6
	TCC_JOB_TIMEOUT         uint32 = 7

	TCC_TASK_CLEAR uint32 = 9
)

type TccTransationListener struct {
	Reserve ReserveHandler //call with RESERVING
	Confirm ConfirmHandler //call with CONFIRMED
	Cancel  CancelHandler  //call with CANCELED
}

func (m *TccTransationListener) OnTransaction(t *Transaction) error {
	switch t.Meta.State {
	case TCC_RESERVING:
		if m.Reserve == nil {
			return fmt.Errorf("no reserve callback")
		}
		return m.Reserve(t)
	case TCC_CONFIRMED:
		if m.Confirm == nil {
			return fmt.Errorf("no confirm callback")
		}
		return m.Confirm(t)
	case TCC_CANCELED:
		if m.Cancel == nil {
			return fmt.Errorf("no cancel callback")
		}
		return m.Cancel(t)
	}
	return fmt.Errorf("state not supported %d", t.Meta.State)

}
