package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

type RequestEventQuery struct {
	TopicQueryObj
}

func (q *RequestEventQuery) QFilter(k, v []byte) bool {
	mf := NewRequestEventFactory()
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
	m, ok := obj.(*protocol.RequestEvent)
	if !ok {
		core.AppLog.Warn().Msgf("wrong request event format %s", err)
		return false
	}
	core.AppLog.Debug().Msgf("filter here %v", m)
	return true
}
