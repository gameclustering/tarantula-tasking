package event

import (
	"fmt"
	"testing"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMessageEventFactory(t *testing.T) {

	me := protocol.MessageEvent{Title: "tile", Message: "msg", DateTime: timestamppb.New(time.Now()), Source: "admin"}
	ptf := NewMessageEventFactory()
	tp, err := ptf.FromMessage(&me,ptf.Header(MESSAGE_EVENT_CID))
	tp.Name = MESSAGE_TOPIC_NAME
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
	}
	tp.NodeId = "nodeId"
	tp.Tag = "presence"
	tp.Name = "message"
	tp.Event.Key.Array = []byte("key1")
	req, err := ptf.Request(tp)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
	}
	tpc, err := ptf.Topic(req.Data.Value)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
	}
	if tpc.Name != tp.Name {
		t.Errorf("Name should be same %s %s", tpc.Name, tp.Name)
	}
	if tpc.NodeId != tp.NodeId {
		t.Errorf("node id should be same %s %s", tpc.NodeId, tp.NodeId)
	}
	if tpc.Tag != tp.Tag {
		t.Errorf("tag should be same %s %s", tpc.Tag, tp.Tag)
	}
	ptf.Message(tpc)
	m, err := ptf.Message(tpc)
	if err != nil {
		t.Errorf("should not be err %s", err.Error())
	}
	mx, ok := m.(*protocol.MessageEvent)
	if !ok {
		t.Errorf("should not be message event %v", ok)
	}
	fmt.Printf("me %s\n", mx.Message)
	mq := MessageEventQuery{}
	mq.FactoryId = core.EVENT_FACTORY_ID
	mq.ClassId = MESSAGE_EVENT_CID
	mq.Topic = MESSAGE_TOPIC_NAME

	qt, _ := ptf.Export(&mq)

	q, _ := ptf.Import(qt)
	if mq.ClassId != q.QClassId() {
		t.Errorf("class id should be same %d %d", mq.ClassId, q.QClassId())
	}
	if ptf.Query().QClassId() != MESSAGE_EVENT_CID {
		t.Errorf("class id should be %d %d", ptf.Query().QClassId(), MESSAGE_EVENT_CID)
	}
	if ptf.Query().QTopic() != MESSAGE_TOPIC_NAME {
		t.Errorf("topic should be %s %s", ptf.Query().QTopic(), MESSAGE_TOPIC_NAME)
	}
}
