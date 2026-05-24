package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

type TransactionEventQuery struct {
	TopicQueryObj
}

func (q *TransactionEventQuery) QFilter(k, v []byte) bool {
	mf := NewTransactionEventFactory()
	t, err := mf.Topic(v)
	if err != nil {
		core.AppLog.Warn().Msgf("wrong decode format %s", err)
		return false
	}
	obj, err := mf.Message(t)
	if err != nil {
		core.AppLog.Warn().Msgf("wrong decode format %s", err)
		return false
	}
	m, ok := obj.(*protocol.TransactionEvent)
	if !ok {
		core.AppLog.Warn().Msgf("wrong message event format %s", err)
		return false
	}
	core.AppLog.Debug().Msgf("filter here %v", m)
	return true
}
