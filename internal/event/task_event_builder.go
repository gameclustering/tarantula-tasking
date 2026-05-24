package event

import (
	"time"

	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TaskEventBuilder struct {
	Target           *protocol.TaskEvent
	ValidatorBuilder *JobEventBuilder
	JobBuilder       *JobEventBuilder
}

func NewTaskEventBuilder(meta *protocol.Meta) *TaskEventBuilder {
	return &TaskEventBuilder{Target: &protocol.TaskEvent{Meta: meta}, ValidatorBuilder: NewJobEventBuilder(),JobBuilder: NewJobEventBuilder()}
}

func (t *TaskEventBuilder) Start(ts time.Time) *TaskEventBuilder {
	t.Target.Start = timestamppb.New(ts)
	return t
}

func (t *TaskEventBuilder) End(ts time.Time) *TaskEventBuilder {
	t.Target.End = timestamppb.New(ts)
	return t
}

func (t *TaskEventBuilder) Description(desc string) *TaskEventBuilder {
	t.Target.Description = desc
	return t
}

func (t *TaskEventBuilder) Build() *protocol.TaskEvent {
	t.Target.Validator = t.ValidatorBuilder.Target
	t.Target.Job = t.JobBuilder.Target
	return t.Target
}
