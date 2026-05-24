package clustering

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

type SetData struct {
	*protocol.Data
	Prefix uint32
	Opt    uint32
	Resp   chan *protocol.Response
}

// HELPER METHODS
func (s *SetData) IndexKey() KeyIndex {
	ki := KeyIndex{Prefix: s.Prefix, Header: s.Header, Key: s.Key}
	return ki
}

func (s *SetData) DataKey() ([]byte, error) {
	ksz := len(s.Key)
	if ksz+20 > core.COMPOSIT_KEY_MAX {
		return []byte{}, fmt.Errorf("Key size overflow %d", ksz)
	}
	buffer := core.NewBuffer(core.COMPOSIT_KEY_MAX)
	buffer.WriteUInt32(s.Header.FactoryId)
	buffer.WriteUInt32(s.Header.ClassId)
	buffer.WriteUInt32(uint32(ksz))
	buffer.Write(s.Key)
	buffer.WriteUInt64(s.Header.Revision)
	buffer.Flip()
	return buffer.Read(0)
}
