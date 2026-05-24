package persistence

import (
	"bytes"
	"testing"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
)

func TestLoginObjectFactory(t *testing.T) {
	mf := NewLoginObjectFactory()
	q, ok := mf.Q.(*LoginObjectQuery)
	if !ok {
		t.Errorf("should be query %v", ok)
	}
	if q.QFactoryId() != core.OBJECT_FACTORY_ID {
		t.Errorf("factory id should be %d %d", q.QFactoryId(), core.OBJECT_FACTORY_ID)
	}
	if q.QClassId() != LOGIN_OBJECT_ID {
		t.Errorf("class id should be %d %d", q.QClassId(), LOGIN_OBJECT_ID)
	}
	if q.QTopic() != LOGIN_OBJECT_FACTORY_NAME {
		t.Errorf("factory name should be %s %s", q.QTopic(), LOGIN_OBJECT_FACTORY_NAME)
	}
	mo := protocol.LoginObject{Name: "n100", Password: "p100", SystemId: 100, ReferenceId: 1, Id: 2, AccessControl: 3}
	kv, err := mf.FromMessage(&mo, &protocol.Header{FactoryId: core.OBJECT_FACTORY_ID, ClassId: LOGIN_OBJECT_ID})
	kv.Key.Array = []byte(mo.Name)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
	}
	if kv.Key.Header.FactoryId != core.OBJECT_FACTORY_ID {
		t.Errorf("factory id %d", kv.Key.Header.FactoryId)
	}
	if kv.Key.Header.ClassId != LOGIN_OBJECT_ID {
		t.Errorf("class id %d", kv.Key.Header.ClassId)
	}
	if kv.Key.Header.Updatable {
		t.Errorf("mutable %v", kv.Key.Header.Updatable)
	}
	req, err := mf.Request(kv)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
	}
	kvx, err := mf.Object(req.Data.Value)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
	}
	if !bytes.Equal(kv.Key.Array, kvx.Key.Array) {
		t.Errorf("should be same key")
	}
	if kvx.Key.Header.FactoryId != kv.Key.Header.FactoryId {
		t.Errorf("should be same factory id")
	}
	if kvx.Key.Header.ClassId != kv.Key.Header.ClassId {
		t.Errorf("should be same factory id")
	}
	mox, err := mf.Message(kvx)
	if err != nil {
		t.Errorf("should not be error %s", err.Error())
	}
	moy, ok := mox.(*protocol.LoginObject)
	if !ok {
		t.Errorf("should be login object")
	}
	if moy.Name != mo.Name {
		t.Errorf("name should be same")
	}

}
