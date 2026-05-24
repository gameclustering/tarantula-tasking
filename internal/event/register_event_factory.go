package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewRegisterEventFactory() *RegisterEventFactory {
	mf := RegisterEventFactory{}
	mq := RegisterEventQuery{}
	mq.Id = REGISTER_TOPIC_NAME
	mq.FactoryId = core.EVENT_FACTORY_ID
	mq.ClassId = REGISTER_EVENT_CID
	mq.Topic = REGISTER_TOPIC_NAME
	mf.Q = &mq
	mf.Mt = func() proto.Message { return &protocol.RegisterEvent{} }
	return &mf
}

type RegisterEventFactory struct {
	ProtoTopicFactoryObj
}

func (p *RegisterEventFactory) FromRegisterEvent(e *protocol.RegisterEvent) (*protocol.Topic, error) {
	tpx := protocol.Topic{Name: REGISTER_TOPIC_NAME}
	msg := protocol.Event{Key:&protocol.Key{Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: REGISTER_EVENT_CID}}}
	obj, err := anypb.New(e)
	if err != nil {
		return &tpx, err
	}
	msg.Message = obj
	tpx.Event = &msg
	return &tpx, nil
}
