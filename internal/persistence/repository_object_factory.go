package persistence

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewRepositoryObjectFactory() *RepositoryObjectFactory {
	mf := RepositoryObjectFactory{}

	mf.Mo = func() proto.Message { return &protocol.RepositoryObject{} }

	mq := RepositoryObjectQuery{}
	mq.Id = REPOSITORY_OBJECT_FACTORY_NAME
	mq.FactoryId = core.OBJECT_FACTORY_ID
	mq.ClassId = REPOSITORY_OBJECT_ID
	mq.Topic = REPOSITORY_OBJECT_FACTORY_NAME
	mf.Q = &mq
	return &mf
}

type RepositoryObjectFactory struct {
	ProtoObjectFactoryObj
}

func (p *RepositoryObjectFactory) FromRepositoryObject(repo *protocol.RepositoryObject) (*protocol.KeyValue, error) {
	kv := protocol.KeyValue{}
	kv.Key = &protocol.Key{Header: &protocol.Header{FactoryId: core.OBJECT_FACTORY_ID, ClassId: REPOSITORY_OBJECT_ID, Updatable: true}}
	obj, err := anypb.New(repo)
	if err != nil {
		return &kv, err
	}
	kv.Message = obj
	return &kv, nil
}
