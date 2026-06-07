package core

import (
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
)

type ProtoObjectFactory interface {
	//CompositeKey
	Hash(h MessageHash) uint32
	Request(obj *protocol.KeyValue) (*protocol.Request, error)
	Object(data []byte) (*protocol.KeyValue, error)
	Message(obj *protocol.KeyValue) (proto.Message, error)
	FromMessage(m proto.Message, h *protocol.Header) (*protocol.KeyValue, error)
	Header(cid uint32) *protocol.Header
	QueryFactory
}
