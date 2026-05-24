package protocol

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type OnCast func() proto.Message
type OnTopic func(t *Topic)
type OnEvent func(e *Event)
type OnMessage func(m proto.Message)

type TopicEventListener struct {
	C OnCast
	T OnTopic
	E OnEvent
	M OnMessage
}

func (t *TopicEventListener) OnTopic(topic *Topic) error {
	if t.C == nil {
		return fmt.Errorf("no caster or delegate callback")
	}
	m := t.C()
	err := topic.Event.Message.UnmarshalTo(m)
	if err != nil {
		return err
	}
	if t.T != nil {
		t.T(topic)
	}
	if t.E != nil {
		t.E(topic.Event)
	}
	if t.M != nil {
		t.M(m)
	}
	return nil
}
