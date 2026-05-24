package event

import (
	"time"

	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TransactionEventBuilder struct {
	target *protocol.TransactionEvent
	jb     *JobEventBuilder
}

func NewTransactionEventBuilder(job *JobEventBuilder) *TransactionEventBuilder {
	return &TransactionEventBuilder{jb: job}
}

func (t *TransactionEventBuilder) New(meta *protocol.Meta) *TransactionEventBuilder {
	t.target = &protocol.TransactionEvent{Meta: meta}
	return t
}

func (t *TransactionEventBuilder) Start(ts time.Time) *TransactionEventBuilder {
	t.target.Start = timestamppb.New(ts)
	return t
}

func (t *TransactionEventBuilder) End(ts time.Time) *TransactionEventBuilder {
	t.target.End = timestamppb.New(ts)
	return t
}

func (t *TransactionEventBuilder) Description(desc string) *TransactionEventBuilder {
	t.target.Description = desc
	return t
}
func (t *TransactionEventBuilder) Build() *TransactionEventBuilder {
	t.jb.Target.Transactions = append(t.jb.Target.Transactions, t.target)
	return t
}
