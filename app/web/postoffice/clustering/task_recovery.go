package clustering

import (
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
)

const taskRecoveryDelay = 30 * time.Second

func (c *DataServiceProvider) recoverTasks() {
	core.AppLog.Info().Msgf("recoverTasks: waiting %s for ring to stabilize", taskRecoveryDelay)
	select {
	case <-time.After(taskRecoveryDelay):
	case <-c.shutdown:
		return
	}

	taskIds := c.scanLocalTaskIds()
	core.AppLog.Info().Msgf("recoverTasks: found %d task(s) in local store", len(taskIds))

	recovered := 0
	for _, taskId := range taskIds {
		tr, err := c.load(taskId)
		if err != nil {
			core.AppLog.Warn().Msgf("recoverTasks: load failed task=%d: %s", taskId, err.Error())
			continue
		}
		if tr.resource.Meta.State == protocol.TCC_FINISHED {
			continue
		}
		if !c.isLocalRingOwner(tr.resource.Meta.Prefix) {
			continue
		}
		core.AppLog.Info().Msgf("recoverTasks: reloading task=%d state=%d", taskId, tr.resource.Meta.State)
		c.TManager.Reload(taskId)
		recovered++
	}
	core.AppLog.Info().Msgf("recoverTasks: done recovered=%d scanned=%d", recovered, len(taskIds))
}

func (c *DataServiceProvider) scanLocalTaskIds() []uint64 {
	ids := make([]uint64, 0)
	index := KeyIndex{}
	pre, err := index.lookupPrefix(INDEX_PREFIX)
	if err != nil {
		return ids
	}
	c.Local.Db.View(func(txn *badger.Txn) error {
		op := badger.IteratorOptions{PrefetchSize: 100, PrefetchValues: false, Reverse: false}
		it := txn.NewIterator(op)
		defer it.Close()
		for it.Seek(pre); it.ValidForPrefix(pre); it.Next() {
			item := it.Item()
			k := append([]byte{}, item.Key()...)
			var v []byte
			item.Value(func(val []byte) error {
				v = append(v, val...)
				return nil
			})
			ki := KeyIndex{Header: &protocol.Header{}}
			if err := core.Import(&ki, k, v, 300); err != nil {
				continue
			}
			if ki.Header.FactoryId != core.TASK_FACTORY_ID || ki.Header.ClassId != persistence.TASK_CLASS_ID {
				continue
			}
			if ki.Header.State == core.DATA_STATE_DELETED {
				continue
			}
			if len(ki.Key) != 8 {
				continue
			}
			buff := core.NewBuffer(8)
			buff.Write(ki.Key)
			buff.Flip()
			taskId, err := buff.ReadUInt64()
			if err != nil {
				continue
			}
			ids = append(ids, taskId)
		}
		return nil
	})
	return ids
}

func (c *DataServiceProvider) isLocalRingOwner(prefix uint32) bool {
	rq := make(chan []core.Node, 3)
	defer close(rq)
	c.MRequest <- core.RingRequest{Opt: REPLICA_RING_OPT, Token: prefix, Replicas: REPLICA_MAX, Async: rq}
	nodes := <-rq
	return len(nodes) > 0 && nodes[0].RpcEndpoint == c.rpcEndpoint
}
