package util

import (
	"errors"
	"sync"
	"time"
)

const (
	BITS_OF_NODE_NUMBER = 10
	BITS_OF_SEQUENCE    = 12
	MAX_NODE_ID         = (1 << BITS_OF_NODE_NUMBER) - 1 //1023
	MAX_SEQUENCE        = (1 << BITS_OF_SEQUENCE) - 1    //4095
)

type Snowflake struct {
	NodeId     int64
	EpochStart int64

	lastTimestamp int64
	sequence      int64

	lock *sync.Mutex
}

func NewSnowflake(nodeId int64, epochStart int64) Snowflake {
	sfk := Snowflake{NodeId: nodeId, EpochStart: epochStart, lastTimestamp: -1, sequence: 0}
	sfk.lock = &sync.Mutex{}
	return sfk
}

func (s *Snowflake) Id() (int64, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	currentTimestamp := time.Now().UnixMilli() - s.EpochStart
	if currentTimestamp < s.lastTimestamp {
		return -1, errors.New("wrong system clock setting")
	}
	if currentTimestamp == s.lastTimestamp {
		s.sequence = (s.sequence + 1) & MAX_SEQUENCE
		if s.sequence == 0 {
			for currentTimestamp == s.lastTimestamp {
				currentTimestamp = time.Now().UnixMilli() - s.EpochStart
			}
		}
	} else {
		s.sequence = 0
	}
	s.lastTimestamp = currentTimestamp
	id := currentTimestamp<<(BITS_OF_NODE_NUMBER+BITS_OF_SEQUENCE) | (s.NodeId << BITS_OF_SEQUENCE) | s.sequence
	return id, nil
}

func (s *Snowflake) Parse(snowflakeId int64) (int64, int64, int64) {
	nodeIdMask := ((1 << BITS_OF_NODE_NUMBER) - 1) << BITS_OF_SEQUENCE
	sequenceMask := (1 << BITS_OF_SEQUENCE) - 1
	timestamp := (snowflakeId >> (BITS_OF_NODE_NUMBER + BITS_OF_SEQUENCE))
	nodeId := (snowflakeId & int64(nodeIdMask)) >> BITS_OF_SEQUENCE
	sequence := snowflakeId & int64(sequenceMask)
	return timestamp + s.EpochStart, nodeId, sequence
}

func (c *Snowflake) UId() uint64 {
	for {
		id, err := c.Id()
		if err == nil {
			return uint64(id)
		}
		time.Sleep(1 * time.Millisecond)
	}
}
