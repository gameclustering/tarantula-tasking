package persistence

import (
	"fmt"

	"gameclustering.com/internal/core"
)

type ObjectQueryObj struct {
	core.QueryObj
}

func (q *ObjectQueryObj) QList(list core.List) error {
	if err := q.QueryObj.QList(list); err != nil {
		return err
	}
	ref, ok := core.QueryFactoryRegistry[q.Id]
	if !ok {
		return fmt.Errorf("query factory not registered %s", q.Id)
	}
	mq := ref()
	mf, ok := mq.(core.ProtoObjectFactory)
	if !ok {
		return fmt.Errorf("wrong query facory with %s", q.Id)
	}
	for _, data := range q.Payload.Data.List {
		kv, err := mf.Object(data.Value)
		if err != nil {
			continue
		}
		core.AppLog.Debug().Msgf("key value : %v", kv)
		e, err := mf.Message(kv)
		if err != nil {
			continue
		}
		if !list(kv.Key.Header, e) {
			break
		}
	}
	return nil
}
