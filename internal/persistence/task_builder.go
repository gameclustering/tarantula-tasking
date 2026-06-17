package persistence

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
)

const (
	TASK_CLASS_ID uint32 = 1
)

type TaskBuilder struct {
	Target *protocol.Task
	key    []byte
	vb     *JobBuilder
}

func NewTaskBuilder(meta *protocol.Meta) *TaskBuilder {
	return &TaskBuilder{Target: &protocol.Task{Meta: meta, Jobs: make([]*protocol.Job, 0)}, vb: NewValidatorBuilder()}
}

func (b *TaskBuilder) Job(meta *protocol.Meta) *JobBuilder {
	return NewJobBuilder(b, meta)
}

func (b *TaskBuilder) Validator(meta *protocol.Meta) *JobBuilder {
	b.vb.Target.Meta = meta
	return b.vb
}

func (b *TaskBuilder) addJob(jb *JobBuilder) {
	b.Target.Jobs = append(b.Target.Jobs, jb.Target)
}

// query task
func (b *TaskBuilder) Build() *protocol.Task {
	b.Target.Validator = b.vb.Build()
	return b.Target
}

// query request
func (b *TaskBuilder) Request() (*protocol.Request, error) {
	req := protocol.Request{Opt: core.CREATE_DATA_REQUEST, Data: &protocol.Data{}}
	if b.Target.Meta.Id == 0 {
		return &req, fmt.Errorf("task id should be more than zero uint64")
	}
	buff := core.NewBuffer(8)
	buff.WriteUInt64(b.Target.Meta.Id)
	buff.Flip()
	key, err := buff.Read(0)
	if err != nil {
		return &req, err
	}
	b.key = key
	value, err := proto.Marshal(b.Target)
	if err != nil {
		return &req, err
	}
	req.Data.Header = &protocol.Header{FactoryId: core.TASK_FACTORY_ID, ClassId: TASK_CLASS_ID, Updatable: true}
	req.Data.Key = key
	req.Data.Value = value
	return &req, nil
}

// parse
func (b *TaskBuilder) From(data []byte) (*protocol.Task, error) {
	b.Target = &protocol.Task{}
	err := proto.Unmarshal(data, b.Target)
	return b.Target, err
}

func (b *TaskBuilder) Hash(h core.MessageHash) uint32 {
	hash := h.RingToken(b.key)
	b.Target.Meta.Prefix = hash
	return hash
}

// HashRequest computes the ring prefix, propagates it to all sub-objects, and
// re-serializes the task so replicated bytes include the correct prefix.
func (b *TaskBuilder) HashRequest(h core.MessageHash) (*protocol.Request, error) {
	req, err := b.Request()
	if err != nil {
		return req, err
	}
	prefix := b.Hash(h)
	if b.Target.Validator != nil {
		b.Target.Validator.Meta.Prefix = prefix
		for _, t := range b.Target.Validator.Transactions {
			t.Meta.Prefix = prefix
		}
	}
	for _, job := range b.Target.Jobs {
		job.Meta.Prefix = prefix
		for _, t := range job.Transactions {
			t.Meta.Prefix = prefix
		}
	}
	value, err := proto.Marshal(b.Target)
	if err != nil {
		return req, err
	}
	req.Data.Value = value
	req.Prefix = prefix
	return req, nil
}
