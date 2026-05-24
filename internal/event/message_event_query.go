package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

type MessageEventQuery struct {
	TopicQueryObj
}

func (q *MessageEventQuery) QFilter(k, v []byte) bool {
	mf := NewMessageEventFactory()
	t, err := mf.Topic(v)
	if err != nil {
		core.AppLog.Warn().Msgf("wrong decode format %s", err)
		return false
	}
	core.AppLog.Debug().Msgf("topic %v", t.Event.Key.Header)
	obj, err := mf.Message(t)
	if err != nil {
		core.AppLog.Warn().Msgf("wrong decode format %s", err)
		return false
	}
	m, ok := obj.(*protocol.MessageEvent)
	if !ok {
		core.AppLog.Warn().Msgf("wrong message event format %s", err)
		return false
	}
	core.AppLog.Debug().Msgf("filter here %v", m)
	return true
}
