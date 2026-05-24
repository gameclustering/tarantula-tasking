package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewMessageEventFactory() *MessageEventFactory {
	mf := MessageEventFactory{}
	mq := MessageEventQuery{}
	mq.Id = MESSAGE_TOPIC_NAME
	mq.FactoryId = core.EVENT_FACTORY_ID
	mq.ClassId = MESSAGE_EVENT_CID
	mq.Topic = MESSAGE_TOPIC_NAME
	mf.Q = &mq
	mf.Mt = func() proto.Message { return &protocol.MessageEvent{} }
	return &mf
}

type MessageEventFactory struct {
	ProtoTopicFactoryObj
}

func (p *MessageEventFactory) FromMessageEvent(e *protocol.MessageEvent) (*protocol.Topic, error) {
	tpx := protocol.Topic{Name: MESSAGE_TOPIC_NAME}
	msg := protocol.Event{Key: &protocol.Key{Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: MESSAGE_EVENT_CID}}}
	obj, err := anypb.New(e)
	if err != nil {
		return &tpx, err
	}
	msg.Message = obj
	tpx.Event = &msg
	return &tpx, nil
}
