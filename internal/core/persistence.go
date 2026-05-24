package core

import "time"

const (
	EVENT_FACTORY_ID  uint32 = 1
	OBJECT_FACTORY_ID uint32 = 2
	TASK_FACTORY_ID   uint32 = 3
	JOB_FACTORY_ID    uint32 = 4
)

func ToBytes(seq Sequence) []byte {
	for {
		id, err := seq.Id()
		if err != nil {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		buff := NewBuffer(8)
		buff.WriteUInt64(uint64(id))
		buff.Flip()
		key, _ := buff.Read(0)
		return key
	}
}


func Export(obj Persistentable, buffSize int) ([]byte, []byte, error) {
	buff := NewBuffer(buffSize)
	var k []byte
	var v []byte
	if err := obj.WriteKey(buff); err != nil {
		return k, v, err
	}
	buff.Flip()
	k, err := buff.Read(0)
	if err != nil {
		return k, v, err
	}
	buff.Clear()
	if err := obj.Write(buff); err != nil {
		return k, v, err
	}
	buff.Flip()
	v, err = buff.Read(0)
	if err != nil {
		return k, v, err
	}
	return k, v, nil
}

func Import(obj Persistentable, k, v []byte, buffSize int) error {
	buff := NewBuffer(buffSize)
	if err := buff.Write(k); err != nil {
		return err
	}
	buff.Flip()
	if err := obj.ReadKey(buff); err != nil {
		return err
	}
	buff.Clear()
	if err := buff.Write(v); err != nil {
		return err
	}
	buff.Flip()
	if err := obj.Read(buff); err != nil {
		return err
	}
	return nil
}

type CompositeKey interface {
	WriteKey(key DataBuffer) error
	ReadKey(key DataBuffer) error
}

type Persistentable interface {
	CompositeKey
	Write(value DataBuffer) error

	Read(value DataBuffer) error
	FactoryId() uint32
	ClassId() uint32

	Revision() uint64
	Timestamp() uint64
	OnTimestamp(tsp uint64)
	OnRevision(rev uint64)
}

type PersistentableObj struct {
	Rev uint64 `json:"rev,string"`
	Tsp uint64 `json:"timestamp,string"`
}

type Stream func(k, v DataBuffer) bool

func (s *PersistentableObj) Write(value DataBuffer) error {
	return nil
}

func (s *PersistentableObj) WriteKey(value DataBuffer) error {
	return nil
}

func (s *PersistentableObj) Read(value DataBuffer) error {
	return nil
}

func (s *PersistentableObj) ReadKey(value DataBuffer) error {
	return nil
}

func (s *PersistentableObj) FactoryId() uint32 {
	return 0
}

func (s *PersistentableObj) ClassId() uint32 {
	return 0
}

func (s *PersistentableObj) Revision() uint64 {
	return s.Rev
}
func (s *PersistentableObj) Timestamp() uint64 {
	return s.Tsp
}
func (s *PersistentableObj) OnTimestamp(tsp uint64) {
	s.Tsp = tsp
}

func (s *PersistentableObj) OnRevision(rev uint64) {
	s.Rev = rev
}
