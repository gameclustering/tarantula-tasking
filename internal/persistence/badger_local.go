package persistence

import (
	"time"

	"gameclustering.com/internal/core"

	badger "github.com/dgraph-io/badger/v4"
)

const (
	BDG_GC_TICK int = 10
)

type BadgerLocal struct {
	InMemory    bool
	Path        string
	Db          *badger.DB
	LogDisabled bool
	GcEnabled   bool
	GcInterval  int
	gcTick      *time.Ticker
}

func (s *BadgerLocal) Open() error {
	var opt badger.Options
	if s.InMemory {
		opt = badger.DefaultOptions("").WithInMemory(true)
		if s.LogDisabled {
			opt.Logger = nil
		}
	} else {
		opt = badger.DefaultOptions(s.Path)
		opt.SyncWrites = false
		if s.LogDisabled {
			opt.Logger = nil
		}
	}
	db, err := badger.Open(opt)
	if err != nil {
		return err
	}
	s.Db = db
	if !s.GcEnabled {
		return nil
	}
	if s.GcInterval == 0 {
		s.GcInterval = BDG_GC_TICK
	}
	s.gcTick = time.NewTicker(time.Duration(s.GcInterval) * time.Minute)
	go func() {
		for range s.gcTick.C {
		gc:
			core.AppLog.Warn().Msgf("running gc at %v", time.Now())
			err := s.Db.RunValueLogGC(0.7)
			if err == nil {
				goto gc
			}
		}
	}()
	return nil
}

func (s *BadgerLocal) Close() error {
	if s.GcEnabled {
		s.gcTick.Stop()
	}
	if s.InMemory {
		return s.Db.Close()
	}
	if s.Db.IsClosed() {
		core.AppLog.Warn().Msg("local db already closed")
		return nil
	}
	s.Db.Sync()
	return s.Db.Close()
}
