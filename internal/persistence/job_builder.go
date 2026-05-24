package persistence

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
)

const (
	JOB_CLASS_ID uint32 = 1
)

type JobBuilder struct {
	tb     *TaskBuilder
	Target *protocol.Job
}

func NewJobBuilder(tb *TaskBuilder, meta *protocol.Meta) *JobBuilder {
	return &JobBuilder{tb: tb, Target: &protocol.Job{Meta: meta, Transactions: make([]*protocol.Transaction, 0)}}
}
func NewValidatorBuilder() *JobBuilder {
	return &JobBuilder{Target: &protocol.Job{Transactions: make([]*protocol.Transaction, 0)}}
}

func (jb *JobBuilder) Transaction() *TransactionBuilder {
	return NewTransactionBuilder(jb)
}

func (jb *JobBuilder) add(t *protocol.Transaction) *JobBuilder {
	jb.Target.Transactions = append(jb.Target.Transactions, t)
	return jb
}

func (jb *JobBuilder) Build() *protocol.Job {
	if jb.tb == nil {
		return jb.Target
	}
	jb.tb.addJob(jb)
	return jb.Target
}

func (b *JobBuilder) Request() (*protocol.Request, error) {
	req := protocol.Request{Opt: core.CREATE_DATA_REQUEST, Data: &protocol.Data{}}
	if b.Target.Meta.Id <= 0 {
		return &req, fmt.Errorf("job id should be more than zero uint64")
	}
	buff := core.NewBuffer(8)
	buff.WriteUInt64(b.Target.Meta.Id)
	buff.Flip()
	key, err := buff.Read(0)
	if err != nil {
		return &req, err
	}
	value, err := proto.Marshal(b.Target)
	if err != nil {
		return &req, err
	}
	req.Data.Header = &protocol.Header{FactoryId: core.JOB_FACTORY_ID, ClassId: JOB_CLASS_ID, Updatable: true}
	req.Data.Key = key
	req.Data.Value = value
	return &req, nil
}

// parse
func (b *JobBuilder) From(data []byte) (*protocol.Job, error) {
	b.Target = &protocol.Job{}
	err := proto.Unmarshal(data, b.Target)
	return b.Target, err
}
