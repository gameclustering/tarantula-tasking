package protocol

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestEvent(t *testing.T) {
	obj, err := anypb.New(&MessageEvent{Title: "test", Message: "message", Source: "any"})
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}
	e := Event{Key:&Key{Header: &Header{FactoryId: 1, ClassId: 1}}, Message: obj}
	data, err := proto.Marshal(&e)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}

	var ex Event
	err = proto.Unmarshal(data, &ex)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}
	var me MessageEvent
	err = ex.Message.UnmarshalTo(&me)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}

	tp := Topic{NodeId: "pd.1", Tag: "presence", Name: "message", Event: &e}
	tdd, err := proto.Marshal(&tp)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}

	var tpx Topic
	err = proto.Unmarshal(tdd, &tpx)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}
}

func TestLoginObject(t *testing.T) {
	login := LoginObject{Name: "p100", Password: "password", SystemId: 100, Id: 2, ReferenceId: 1, AccessControl: 0}
	k := Key{Array: []byte(login.Name), Header: &Header{FactoryId: 10, ClassId: 2, Updatable: true}}
	v, err := anypb.New(&login)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}
	kv := KeyValue{Key: &k, Message: v}
	bs, err := proto.Marshal(&kv)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}
	var pkv KeyValue
	err = proto.Unmarshal(bs, &pkv)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}
	var loginx LoginObject
	err = pkv.Message.UnmarshalTo(&loginx)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
		return
	}
	fmt.Printf("kv %v\n", &pkv)
	fmt.Printf("obj %v\n", &loginx)
}
