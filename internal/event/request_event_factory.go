package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewRequestEventFactory() *RequestEventFactory {
	mf := RequestEventFactory{}
	mq := RequestEventQuery{}
	mq.Id = REQUEST_TOPIC_NAME
	mq.FactoryId = core.EVENT_FACTORY_ID
	mq.ClassId = REQUEST_EVENT_CID
	mq.Topic = REQUEST_TOPIC_NAME
	mf.Q = &mq
	mf.Mt = func() proto.Message { return &protocol.RequestEvent{} }
	return &mf
}

type RequestEventFactory struct {
	ProtoTopicFactoryObj
}

func (p *RequestEventFactory) FromRequestEvent(e *protocol.RequestEvent) (*protocol.Topic, error) {
	tpx := protocol.Topic{Name: REQUEST_TOPIC_NAME}
	msg := protocol.Event{Key:&protocol.Key{Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: REQUEST_EVENT_CID}}}
	obj, err := anypb.New(e)
	if err != nil {
		return &tpx, err
	}
	msg.Message = obj
	tpx.Event = &msg
	return &tpx, nil
}
