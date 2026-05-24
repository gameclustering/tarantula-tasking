package core

import (
	"bytes"
	"encoding/binary"

	buffer "github.com/0xc0d/encoding/bytebuffer"
)

func NewBuffer(size int) DataBuffer {
	bf := BufferProxy{}
	bf.NewProxy(size)
	return &bf
}

type BufferProxy struct {
	data *buffer.ByteBuffer
}

func (s *BufferProxy) NewProxy(size int) {
	s.data = buffer.NewByteBuffer(size)
	s.data.SetOrder(binary.BigEndian)
}

func (s *BufferProxy) WriteBool(data bool) error {
	if data {
		return s.data.Put(byte(1))
	}
	return s.data.Put(byte(0))
}

func (s *BufferProxy) WriteComplex64(data complex64) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())
}

func (s *BufferProxy) WriteComplex128(data complex128) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())
}

func (s *BufferProxy) WriteFloat64(data float64) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())
}

func (s *BufferProxy) WriteFloat32(data float32) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())
}

func (s *BufferProxy) WriteInt64(data int64) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())
}

func (s *BufferProxy) WriteUInt64(data uint64) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())
}

func (s *BufferProxy) WriteInt32(data int32) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())

}

func (s *BufferProxy) WriteUInt32(data uint32) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())

}

func (s *BufferProxy) WriteInt16(data int16) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())
}

func (s *BufferProxy) WriteInt8(data int8) error {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, data)
	if err != nil {
		return err
	}
	return s.data.PutBytes(buf.Bytes(), 0, buf.Len())

}

func (s *BufferProxy) WriteString(data string) error {
	slen := len(data)
	err := s.WriteInt32(int32(slen))
	if err != nil {
		return err
	}
	return s.data.PutBytes([]byte(data), 0, slen)

}

func (s *BufferProxy) ReadInt32() (int32, error) {
	buf := make([]byte, 4)
	err := s.data.GetBytes(buf, 0, 4)
	if err != nil {
		return 0, err
	}
	var v int32
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadUInt32() (uint32, error) {
	buf := make([]byte, 4)
	err := s.data.GetBytes(buf, 0, 4)
	if err != nil {
		return 0, err
	}
	var v uint32
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadInt64() (int64, error) {
	buf := make([]byte, 8)
	err := s.data.GetBytes(buf, 0, 8)
	if err != nil {
		return 0, err
	}
	var v int64
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadUInt64() (uint64, error) {
	buf := make([]byte, 8)
	err := s.data.GetBytes(buf, 0, 8)
	if err != nil {
		return 0, err
	}
	var v uint64
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}
func (s *BufferProxy) ReadFloat32() (float32, error) {
	buf := make([]byte, 4)
	err := s.data.GetBytes(buf, 0, 4)
	if err != nil {
		return 0, err
	}
	var v float32
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadFloat64() (float64, error) {
	buf := make([]byte, 8)
	err := s.data.GetBytes(buf, 0, 8)
	if err != nil {
		return 0, err
	}
	var v float64
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadInt16() (int16, error) {
	buf := make([]byte, 2)
	err := s.data.GetBytes(buf, 0, 2)
	if err != nil {
		return 0, err
	}
	var v int16
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadInt8() (int8, error) {
	buf := make([]byte, 1)
	err := s.data.GetBytes(buf, 0, 1)
	if err != nil {
		return 0, err
	}
	var v int8
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadComplex64() (complex64, error) {
	buf := make([]byte, 8)
	err := s.data.GetBytes(buf, 0, 8)
	if err != nil {
		return 0, err
	}
	var v complex64
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadComplex128() (complex128, error) {
	buf := make([]byte, 16)
	err := s.data.GetBytes(buf, 0, 16)
	if err != nil {
		return 0, err
	}
	var v complex128
	err = binary.Read(bytes.NewBuffer(buf), binary.BigEndian, &v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (s *BufferProxy) ReadString() (string, error) {
	len, err := s.ReadInt32()
	if err != nil {
		return "", err
	}
	buf := make([]byte, len)
	err = s.data.GetBytes(buf, 0, int(len))
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (s *BufferProxy) ReadBool() (bool, error) {
	b, err := s.data.Get()
	if err != nil {
		return false, err
	}
	return int(b) == 1, nil
}
func (s *BufferProxy) Read(sz int) ([]byte, error) {
	len := s.data.Remaining()
	if sz == 0 || sz >= len {
		k := make([]byte, len)
		err := s.data.GetBytes(k, 0, len)
		if err != nil {
			return k, err
		}
		return k, nil
	}
	k := make([]byte, sz)
	err := s.data.GetBytes(k, 0, sz)
	if err != nil {
		return k, err
	}
	return k, nil
}

func (s *BufferProxy) Export(delimit byte) ([]byte, error) {
	sz := s.data.Remaining()
	v := make([]byte, sz+1)
	err := s.data.GetBytes(v, 0, sz)
	if err != nil {
		return v, err
	}
	v[sz] = delimit
	return v, nil
}

func (s *BufferProxy) Write(data []byte) error {
	return s.data.PutBytes(data, 0, len(data))
}

func (s *BufferProxy) Remaining() int {
	return s.data.Remaining()
}

func (s *BufferProxy) Flip() error {
	return s.data.Flip()
}

func (s *BufferProxy) Rewind() error {
	return s.data.Rewind()
}

func (s *BufferProxy) Clear() error {
	return s.data.Clear()
}
