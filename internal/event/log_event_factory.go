package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewLogEventFactory() *LogEventFactory {
	mf := LogEventFactory{}
	mq := LogEventQuery{}
	mq.Id = LOG_TOPIC_NAME
	mq.FactoryId = core.EVENT_FACTORY_ID
	mq.ClassId = LOG_EVENT_CID
	mq.Topic = LOG_TOPIC_NAME
	mf.Q = &mq
	mf.Mt = func() proto.Message { return &protocol.LogEvent{} }
	return &mf
}

type LogEventFactory struct {
	ProtoTopicFactoryObj
}

func (p *LogEventFactory) FromLogEvent(e *protocol.LogEvent) (*protocol.Topic, error) {
	tpx := protocol.Topic{Name: LOG_TOPIC_NAME}
	msg := protocol.Event{Key:&protocol.Key{Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: LOG_EVENT_CID}}}
	obj, err := anypb.New(e)
	if err != nil {
		return &tpx, err
	}
	msg.Message = obj
	tpx.Event = &msg
	return &tpx, nil
}
