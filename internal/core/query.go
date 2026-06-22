package core

import (
	"time"

	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
)

type List func(h *protocol.Header, m proto.Message) bool

type Query interface {
	QId() string
	QFactoryId() uint32
	QClassId() uint32
	QNodeId() string
	QTag() string
	QTopic() string
	QStartTime() time.Time
	QEndTime() time.Time
	QLimit() int32 
	QOffset() int32
	QRead(b DataBuffer) error
	QWrite(b DataBuffer) error
	QFilter(k, v []byte) bool

	//read
	QList(list List) error
	QResponse(resp *protocol.Response)
	//query target ring
	Hash(h MessageHash) uint32
}

type QueryFactory interface {
	Export(query Query) ([]byte, error)
	Import(criteria []byte) (Query, error)
	Query() Query
	Set(resp *protocol.Response) Query
}
