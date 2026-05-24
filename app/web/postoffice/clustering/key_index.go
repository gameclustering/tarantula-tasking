package clustering

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

const (
	INDEX_PREFIX string = "$___KI___$"
)

type KeyIndex struct {
	Prefix uint32           `json:"prefix"`
	Key    []byte           `json:"-"`
	Header *protocol.Header `json:"header"`
	Nodes  []uint32         `json:"-"`

	core.PersistentableObj `json:"-"`
}

func (k *KeyIndex) WriteKey(buffer core.DataBuffer) error {
	if err := buffer.WriteString(INDEX_PREFIX); err != nil {
		return err
	}
	if err := buffer.WriteUInt32(k.Prefix); err != nil {
		return err
	}
	if err := buffer.WriteUInt32(k.Header.FactoryId); err != nil {
		return err
	}
	if err := buffer.WriteUInt32(k.Header.ClassId); err != nil {
		return err
	}
	if err := buffer.WriteInt32(int32(len(k.Key))); err != nil {
		return err
	}
	if err := buffer.Write(k.Key); err != nil {
		return err
	}
	return nil
}
func (k *KeyIndex) Write(buffer core.DataBuffer) error {
	if err := buffer.WriteUInt64(k.Header.Revision); err != nil {
		return err
	}
	if err := buffer.WriteUInt64(k.Header.Timestamp); err != nil {
		return err
	}
	if err := buffer.WriteUInt32(k.Header.Size); err != nil {
		return err
	}
	if err := buffer.WriteBool(k.Header.Updatable); err != nil {
		return err
	}
	if err := buffer.WriteUInt32(k.Header.State); err != nil {
		return err
	}
	return nil
}
func (k *KeyIndex) Read(buffer core.DataBuffer) error {
	rev, err := buffer.ReadUInt64()
	if err != nil {
		return err
	}
	k.Header.Revision = rev
	ts, err := buffer.ReadUInt64()
	if err != nil {
		return err
	}
	k.Header.Timestamp = ts
	sz, err := buffer.ReadUInt32()
	if err != nil {
		return err
	}
	k.Header.Size = sz
	mtu, err := buffer.ReadBool()
	if err != nil {
		return err
	}
	k.Header.Updatable = mtu
	st, err := buffer.ReadUInt32()
	if err != nil {
		return err
	}
	k.Header.State = st
	return nil

}
func (k *KeyIndex) ReadKey(buffer core.DataBuffer) error {
	_, err := buffer.ReadString()
	if err != nil {
		return err
	}
	prefix, err := buffer.ReadUInt32()
	if err != nil {
		return err
	}
	k.Prefix = prefix
	fid, err := buffer.ReadUInt32()
	if err != nil {
		return err
	}
	k.Header.FactoryId = fid
	cid, err := buffer.ReadUInt32()
	if err != nil {
		return err
	}
	k.Header.ClassId = cid
	klen, err := buffer.ReadInt32()
	if err != nil {
		return err
	}
	key, err := buffer.Read(int(klen))
	if err != nil {
		return err
	}
	k.Key = key
	return nil
}

//HELP METHODS

func (k *KeyIndex) Val(val []byte) error {
	kBuff := core.NewBuffer(core.COMPOSIT_KEY_MAX)
	if err := kBuff.Write(val); err != nil {
		return err
	}
	kBuff.Flip()
	return k.Read(kBuff)
}

func (k *KeyIndex) CompositKey() ([]byte, error) {
	kBuff := core.NewBuffer(core.COMPOSIT_KEY_MAX)
	if err := k.WriteKey(kBuff); err != nil {
		return nil, err
	}
	kBuff.Flip()
	return kBuff.Read(0)
}
func (k *KeyIndex) Value() ([]byte, error) {
	kBuff := core.NewBuffer(core.COMPOSIT_KEY_MAX)
	if err := k.Write(kBuff); err != nil {
		return nil, err
	}
	kBuff.Flip()
	return kBuff.Read(0)
}
func (k *KeyIndex) Pair() ([]byte, []byte, error) {
	kBuff := core.NewBuffer(core.COMPOSIT_KEY_MAX)
	if err := k.WriteKey(kBuff); err != nil {
		return nil, nil, err
	}
	kBuff.Flip()
	key, err := kBuff.Read(0)
	if err != nil {
		return nil, nil, err
	}
	kBuff.Clear()
	if err := k.Write(kBuff); err != nil {
		return nil, nil, err
	}
	kBuff.Flip()
	value, err := kBuff.Read(0)
	if err != nil {
		return nil, nil, err
	}
	return key, value, nil
}

func (k *KeyIndex) lookupPrefix(p string) ([]byte, error) {
	buff := core.NewBuffer(core.COMPOSIT_KEY_MAX)
	buff.WriteString(p)
	buff.Flip()
	return buff.Read(0)
}

func (k *KeyIndex) lookupDataKey() ([]byte, error) {
	ksz := len(k.Key)
	buffer := core.NewBuffer(core.COMPOSIT_KEY_MAX)
	buffer.WriteUInt32(k.Header.FactoryId)
	buffer.WriteUInt32(k.Header.ClassId)
	buffer.WriteInt32(int32(ksz))
	buffer.Write(k.Key)
	buffer.WriteUInt64(k.Header.Revision)
	buffer.Flip()
	return buffer.Read(0)
}
