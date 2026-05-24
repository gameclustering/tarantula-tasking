package persistence

import (
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/types/known/anypb"
)

type TransactionBuilder struct {
	target *protocol.Transaction
	jb     *JobBuilder
}

func NewTransactionBuilder(jb *JobBuilder) *TransactionBuilder {
	return &TransactionBuilder{jb: jb, target: &protocol.Transaction{}}
}

func (t *TransactionBuilder) Meta(meta *protocol.Meta) *TransactionBuilder {
	t.target.Meta = meta
	return t
}

func (t *TransactionBuilder) Object(obj *protocol.KeyValue) *TransactionBuilder {
	t.target.Object = obj
	return t
}

func (t *TransactionBuilder) Data(data *protocol.Data) *TransactionBuilder {
	t.target.Data = data
	return t
}

func (t *TransactionBuilder) Message(message *anypb.Any) *TransactionBuilder {
	t.target.Message = message
	return t
}

func (t *TransactionBuilder) Build() {
	t.jb.add(t.target)
}
