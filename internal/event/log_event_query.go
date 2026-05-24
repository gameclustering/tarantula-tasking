package event

import (
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

type LogEventQuery struct {
	TopicQueryObj
}

func (q *LogEventQuery) QFilter(k, v []byte) bool {
	mf := NewLogEventFactory()
	t, err := mf.Topic(v)
	if err != nil {
		core.AppLog.Warn().Msgf("wrong decode format %s", err)
		return false
	}
	core.AppLog.Debug().Msgf("topic %v", time.UnixMilli(int64(t.Event.Key.Header.Timestamp)))
	obj, err := mf.Message(t)
	if err != nil {
		core.AppLog.Warn().Msgf("wrong decode format %s", err)
		return false
	}
	m, ok := obj.(*protocol.LogEvent)
	if !ok {
		core.AppLog.Warn().Msgf("wrong log event format %s", err)
		return false
	}
	core.AppLog.Debug().Msgf("filter here %v", m)
	return true
}
