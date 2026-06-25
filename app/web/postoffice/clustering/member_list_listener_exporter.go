package clustering

type MemberListListenerExporter struct {
	
	*MemberListListener
}

func (m *MemberListListenerExporter) RingToken(key []byte) uint32 {
	return m.RingToken(key)
}
