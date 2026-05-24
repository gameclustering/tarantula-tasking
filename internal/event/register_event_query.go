package event

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

type RegisterEventQuery struct {
	TopicQueryObj
}

func (q *RegisterEventQuery) QFilter(k, v []byte) bool {
	mf := NewRegisterEventFactory()
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
	m, ok := obj.(*protocol.RegisterEvent)
	if !ok {
		core.AppLog.Warn().Msgf("wrong register event format %s", err)
		return false
	}
	core.AppLog.Debug().Msgf("filter here %v", m)
	return true
}
