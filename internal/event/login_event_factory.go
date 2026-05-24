package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewLoginEventFactory() *LoginEventFactory {
	mf := LoginEventFactory{}
	mq := LoginEventQuery{}
	mq.Id = LOGIN_TOPIC_NAME
	mq.FactoryId = core.EVENT_FACTORY_ID
	mq.ClassId = LOGIN_EVENT_CID
	mq.Topic = LOGIN_TOPIC_NAME
	mf.Q = &mq
	mf.Mt = func() proto.Message { return &protocol.LoginEvent{} }
	return &mf
}

type LoginEventFactory struct {
	ProtoTopicFactoryObj
}

func (p *LoginEventFactory) FromLoginEvent(e *protocol.LoginEvent) (*protocol.Topic, error) {
	tpx := protocol.Topic{Name: LOGIN_TOPIC_NAME}
	msg := protocol.Event{Key:&protocol.Key{Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: LOGIN_EVENT_CID}}}
	obj, err := anypb.New(e)
	if err != nil {
		return &tpx, err
	}
	msg.Message = obj
	tpx.Event = &msg
	return &tpx, nil
}
