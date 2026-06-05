package persistence

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
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

func (p *LoginObjectFactory) FromLoginObject(login *protocol.LoginObject) (*protocol.Request, error) {
	obj, err := p.FromMessage(login, p.Header(LOGIN_OBJECT_ID))
	if err != nil {
		return nil, err
	}
	obj.Key.Array = []byte(login.Name)
	return p.Request(obj)
}
