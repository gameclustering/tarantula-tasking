package event

import (
	"fmt"

	"gameclustering.com/internal/core"
)

type TopicQueryObj struct {
	core.QueryObj
}

func (q *TopicQueryObj) QList(list core.List) error {
	if err := q.QueryObj.QList(list); err != nil {
		return err
	}
	ref, ok := core.QueryFactoryRegistry[q.Id]
	if !ok {
		return fmt.Errorf("query factory not registered %s", q.Id)
	}
	mq := ref()
	mf, ok := mq.(core.ProtoTopicFactory)
	if !ok {
		return fmt.Errorf("wrong query facory with %s", q.Id)
	}
	for _, data := range q.Payload.Data.List {
		tp, err := mf.Topic(data.Value)
		if err != nil {
			continue
		}
		core.AppLog.Debug().Msgf("topic : %v", tp)
		e, err := mf.Message(tp)
		if err != nil {
			continue
		}
		if !list(tp.Event.Key.Header, e) {
			break
		}
	}
	return nil
}
