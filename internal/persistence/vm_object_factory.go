package persistence

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewVMObjectFactory() *VMObjectFactory {
	mf := VMObjectFactory{}

	mf.Mo = func() proto.Message { return &protocol.VMObject{} }

	mq := VMObjectQuery{}
	mq.Id = VM_OBJECT_FACTORY_NAME
	mq.FactoryId = core.OBJECT_FACTORY_ID
	mq.ClassId = VM_OBJECT_ID
	mq.Topic = VM_OBJECT_FACTORY_NAME
	mf.Q = &mq
	return &mf
}

type VMObjectFactory struct {
	ProtoObjectFactoryObj
}

func (p *VMObjectFactory) FromVMObject(vmo *protocol.VMObject) (*protocol.KeyValue, error) {
	kv := protocol.KeyValue{}
	kv.Key = &protocol.Key{Header: &protocol.Header{FactoryId: core.OBJECT_FACTORY_ID, ClassId: VM_OBJECT_ID, Updatable: true}}
	obj, err := anypb.New(vmo)
	if err != nil {
		return &kv, err
	}
	kv.Message = obj
	return &kv, nil
}
