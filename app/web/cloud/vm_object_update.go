package main

import (
	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewVMObejctUpdate(s *CloudService) *protocol.TccTransationListener {
	vm := VMObjectUpdate{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = vm.reserse
	tcc.Confirm = vm.confirm
	tcc.Cancel = vm.cancel
	return &tcc
}

type VMObjectUpdate struct {
	*CloudService
}

func (v *VMObjectUpdate) reserse(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update reserve %v", t.Meta)
	var vm protocol.VMObject
	err := anypb.UnmarshalTo(t.Message, &vm, proto.UnmarshalOptions{})
	if err != nil {
		return err
	}
	core.AppLog.Debug().Msgf("vm object %v", &vm)
	return v.insert(t.Meta)
}

func (v *VMObjectUpdate) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *VMObjectUpdate) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("update cancel %v", t.Meta)
	return v.insert(t.Meta)
}
