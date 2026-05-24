package clustering

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

type GetData struct {
	*protocol.Request
}

func (s *GetData) IndexKey() KeyIndex {
	data := s.Data
	ki := KeyIndex{Prefix: s.Prefix, Header: data.Header, Key: data.Key}
	return ki
}
func (s *GetData) DataKey() ([]byte, error) {
	data := s.Data
	ksz := len(data.Key)
	if ksz+20 > core.COMPOSIT_KEY_MAX {
		return []byte{}, fmt.Errorf("Key size overflow %d", ksz)
	}
	buffer := core.NewBuffer(core.COMPOSIT_KEY_MAX)
	buffer.WriteUInt32(data.Header.FactoryId)
	buffer.WriteUInt32(data.Header.ClassId)
	buffer.WriteInt32(int32(ksz))
	buffer.Write(data.Key)
	buffer.WriteUInt64(data.Header.Revision)
	buffer.Flip()
	return buffer.Read(0)
}
