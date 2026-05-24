package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewTransactionEventFactory() *TransactionEventFactory {
	mf := TransactionEventFactory{}
	mq := TransactionEventQuery{}
	mq.Id = TRANSACTION_TOPIC_NAME
	mq.FactoryId = core.EVENT_FACTORY_ID
	mq.ClassId = TRANSACTION_EVENT_CID
	mq.Topic = TRANSACTION_TOPIC_NAME
	mf.Q = &mq
	mf.Mt = func() proto.Message { return &protocol.TransactionEvent{} }
	return &mf
}

type TransactionEventFactory struct {
	ProtoTopicFactoryObj
}

func (p *TransactionEventFactory) GetRequest(key []byte) *protocol.Request {
	return &protocol.Request{Opt: core.GET_DATA_REQUEST, Data: &protocol.Data{Key: key, Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: TRANSACTION_EVENT_CID}}}
}
func (p *TransactionEventFactory) FromTransactionEvent(e *protocol.TransactionEvent) (*protocol.Topic, error) {
	tpx := protocol.Topic{Name: TRANSACTION_TOPIC_NAME}
	msg := protocol.Event{Key: &protocol.Key{Header: &protocol.Header{FactoryId: core.EVENT_FACTORY_ID, ClassId: TRANSACTION_EVENT_CID}}}
	obj, err := anypb.New(e)
	if err != nil {
		return &tpx, err
	}
	msg.Message = obj
	tpx.Event = &msg
	return &tpx, nil
}
