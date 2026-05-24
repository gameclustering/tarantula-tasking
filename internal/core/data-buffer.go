package core

import "fmt"

type DataBuffer interface {
	WriteBool(data bool) error

	WriteComplex64(data complex64) error

	WriteComplex128(data complex128) error

	WriteFloat64(data float64) error

	WriteFloat32(data float32) error

	WriteInt64(data int64) error
	WriteUInt64(data uint64) error
	WriteInt32(data int32) error

	WriteUInt32(data uint32) error

	WriteInt16(data int16) error

	WriteInt8(data int8) error

	WriteString(data string) error

	Write(data []byte) error

	ReadInt32() (int32, error)
	ReadUInt32() (uint32, error)

	ReadInt64() (int64, error)
	ReadUInt64() (uint64, error)
	ReadFloat32() (float32, error)

	ReadFloat64() (float64, error)

	ReadInt16() (int16, error)

	ReadInt8() (int8, error)

	ReadComplex64() (complex64, error)

	ReadComplex128() (complex128, error)

	ReadString() (string, error)

	ReadBool() (bool, error)

	Read(sz int) ([]byte, error)
	Export(delimit byte) ([]byte, error)

	Remaining() int

	Flip() error

	Rewind() error

	Clear() error
}

type DataBufferHook struct{}

func (s *DataBufferHook) WriteBool(data bool) error {
	return nil
}

func (s *DataBufferHook) WriteComplex64(data complex64) error {
	return nil
}
func (s *DataBufferHook) WriteComplex128(data complex128) error {
	return nil
}
func (s *DataBufferHook) WriteFloat64(data float64) error {

	return nil
}

func (s *DataBufferHook) WriteFloat32(data float32) error {
	return nil
}

func (s *DataBufferHook) WriteInt64(data int64) error {
	return nil
}
func (s *DataBufferHook) WriteUInt64(data uint64) error {
	return nil
}
func (s *DataBufferHook) WriteInt32(data int32) error {
	return nil
}

func (s *DataBufferHook) WriteInt16(data int16) error {
	return nil
}

func (s *DataBufferHook) WriteInt8(data int8) error {
	return nil
}

func (s *DataBufferHook) WriteString(data string) error {
	return nil
}

func (s *DataBufferHook) Write(data []byte) error {
	return nil
}

func (s *DataBufferHook) ReadBool() (bool, error) {
	return false, nil
}

func (s *DataBufferHook) ReadInt32() (int32, error) {
	return 0, nil
}

func (s *DataBufferHook) ReadInt64() (int64, error) {
	return 0, nil
}
func (s *DataBufferHook) ReadUInt64() (uint64, error) {
	return 0, nil
}

func (s *DataBufferHook) ReadFloat32() (float32, error) {
	return 0, nil
}

func (s *DataBufferHook) ReadFloat64() (float64, error) {
	return 0, nil
}

func (s *DataBufferHook) ReadInt16() (int16, error) {
	return 0, nil
}

func (s *DataBufferHook) ReadInt8() (int8, error) {
	return 0, nil
}

func (s *DataBufferHook) ReadComplex64() (complex64, error) {
	return 0, nil
}

func (s *DataBufferHook) ReadComplex128() (complex128, error) {
	return 0, nil
}

func (s *DataBufferHook) ReadString() (string, error) {
	return "", nil
}

func (s *DataBufferHook) Read(sz int) ([]byte, error) {
	return []byte{0}, nil
}

func (s *DataBufferHook) Remaining() int {
	return 0
}

func (s *DataBufferHook) Flip() error {
	return nil
}
func (s *DataBufferHook) Rewind() error {
	return nil
}
func (s *DataBufferHook) Clear() error {
	return nil
}
func (s *DataBufferHook) Export(delimit byte) ([]byte, error) {
	return nil, fmt.Errorf("unsupported")
}
