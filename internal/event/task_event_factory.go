package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewTaskEventFactory() *TaskEventFactory {
	mf := TaskEventFactory{}
	mq := TaskEventQuery{}
	mq.Id = TASK_TOPIC_NAME
	mq.FactoryId = core.EVENT_FACTORY_ID
	mq.ClassId = TASK_EVENT_CID
	mq.Topic = TASK_TOPIC_NAME
	mf.Q = &mq
	mf.Mt = func() proto.Message { return &protocol.TaskEvent{} }
	return &mf
}

type TaskEventFactory struct {
	ProtoTopicFactoryObj
}

func (p *TaskEventFactory) GetRequest(key []byte) *protocol.Request {
	return &protocol.Request{Opt: core.GET_DATA_REQUEST, Data: &protocol.Data{Key: key, Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: TASK_EVENT_CID}}}
}
func (p *TaskEventFactory) FromTaskEvent(e *protocol.TaskEvent) (*protocol.Topic, error) {
	tpx := protocol.Topic{Name: TASK_TOPIC_NAME}
	msg := protocol.Event{Key: &protocol.Key{Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: TASK_EVENT_CID}}}
	obj, err := anypb.New(e)
	if err != nil {
		return &tpx, err
	}
	msg.Message = obj
	tpx.Event = &msg
	return &tpx, nil
}
