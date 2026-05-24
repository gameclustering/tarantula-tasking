package event

import (
	"fmt"
	"testing"
	"time"

	"gameclustering.com/internal/protocol"
)

func TestTaskEventBuilder(t *testing.T) {
	tb := NewTaskEventBuilder(&protocol.Meta{Id: 1, Name: "task"}).Description("task")
	vb := tb.ValidatorBuilder
	vb.Meta(&protocol.Meta{Id: 2, Name: "validaor"})
	vb.Transaction().New(&protocol.Meta{Name: "v1"}).Build()
	vb.Transaction().New(&protocol.Meta{Name: "v2"}).Build()
	jb := tb.JobBuilder
	jb.Meta(&protocol.Meta{Id: 4, Name: "job"}).Description("desc")
	jb.Transaction().New(&protocol.Meta{Name: "j1"}).Build()
	jb.Transaction().New(&protocol.Meta{Name: "J2"}).Build()
	jb.Start(time.Now()).End(time.Now())
	vb.Description("validator").End(time.Now()).Start(time.Now().Add(3 * time.Second))
	te := tb.Build()
	if len(te.Validator.Transactions) != 2 {
		t.Errorf("validator should have 2 transactions %d", len(te.Validator.Transactions))
	}
	if len(te.Job.Transactions) != 2 {
		t.Errorf("job should have 2 transactions %d", len(te.Job.Transactions))
	}
	fmt.Printf("Task event %v", te)
	tf := NewTaskEventFactory()
	topic, _ := tf.FromTaskEvent(te)
	topic.Event.Key.Array = []byte("hell")
	fmt.Printf("Topic %v", topic)
	req, _ := tf.Request(topic)
	topicx, _ := tf.Topic(req.Data.Value)
	fmt.Printf("Topicx %v", topicx)
}
