package persistence

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewLoginObjectFactory() *LoginObjectFactory {
	mf := LoginObjectFactory{}

	mf.Mo = func() proto.Message { return &protocol.LoginObject{} }

	mq := LoginObjectQuery{}
	mq.Id = LOGIN_OBJECT_FACTORY_NAME
	mq.FactoryId = core.OBJECT_FACTORY_ID
	mq.ClassId = LOGIN_OBJECT_ID
	mq.Topic = LOGIN_OBJECT_FACTORY_NAME
	mf.Q = &mq
	return &mf
}

type LoginObjectFactory struct {
	ProtoObjectFactoryObj
}

func (p *LoginObjectFactory) FromLoginObject(login *protocol.LoginObject) (*protocol.KeyValue, error) {
	kv := protocol.KeyValue{}
	kv.Key = &protocol.Key{Array: []byte(login.Name), Header: &protocol.Header{FactoryId: core.OBJECT_FACTORY_ID, ClassId: LOGIN_OBJECT_ID, Updatable: true}}
	obj, err := anypb.New(login)
	if err != nil {
		return &kv, err
	}
	kv.Message = obj
	return &kv, nil
}
