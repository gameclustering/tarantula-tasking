package core

import (
	"gameclustering.com/internal/protocol"
)

type QueryFactoryObj struct {
	Q Query
}

func (f *QueryFactoryObj) Import(data []byte) (Query, error) {
	buff := NewBuffer(QUERY_SIZE_MAX)
	if err := buff.Write(data); err != nil {
		return f.Q, err
	}
	buff.Flip()
	if err := f.Q.QRead(buff); err != nil {
		return f.Q, err
	}
	return f.Q, nil
}
func (f *QueryFactoryObj) Export(query Query) ([]byte, error) {
	var v []byte
	buff := NewBuffer(QUERY_SIZE_MAX)
	if err := query.QWrite(buff); err != nil {
		return v, nil
	}
	buff.Flip()
	return buff.Read(0)
}

func (f *QueryFactoryObj) Query() Query {
	return f.Q
}

func (f *QueryFactoryObj) Set(resp *protocol.Response) Query {
	f.Q.QResponse(resp)
	return f.Q
}

func (f *QueryFactoryObj) Hash(h MessageHash) uint32{
	return 0
}
