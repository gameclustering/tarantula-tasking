package event

import (
	"time"

	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type JobEventBuilder struct {
	Target *protocol.JobEvent
	tb     *TransactionEventBuilder
}

func NewJobEventBuilder() *JobEventBuilder{
	return &JobEventBuilder{Target: &protocol.JobEvent{Transactions: make([]*protocol.TransactionEvent, 0)}}
}

func (t *JobEventBuilder) Meta(meta *protocol.Meta) *JobEventBuilder {
	t.Target.Meta =  meta
	return t
}

func (t *JobEventBuilder) Start(ts time.Time) *JobEventBuilder {
	t.Target.Start = timestamppb.New(ts)
	return t
}

func (t *JobEventBuilder) End(ts time.Time) *JobEventBuilder {
	t.Target.End = timestamppb.New(ts)
	return t
}

func (t *JobEventBuilder) Description(desc string) *JobEventBuilder {
	t.Target.Description = desc
	return t
}

// chaining build a transaction New to Build
func (t *JobEventBuilder) Transaction() *TransactionEventBuilder {
	t.tb = NewTransactionEventBuilder(t)
	return t.tb
}
