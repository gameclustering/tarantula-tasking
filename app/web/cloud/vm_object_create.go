package main

import (
	"fmt"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/protocol"
	"gameclustering.com/internal/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func NewVMObjectCreate(s *CloudService) *protocol.TccTransationListener {
	vm := VMObjectCreate{s}
	tcc := protocol.TccTransationListener{}
	tcc.Reserve = vm.reserve
	tcc.Confirm = vm.confirm
	tcc.Cancel = vm.cancel
	return &tcc
}

type VMObjectCreate struct {
	*CloudService
}

func (v *VMObjectCreate) reserve(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create reserve %v", t.Meta)
	var vm protocol.VMObject
	if err := anypb.UnmarshalTo(t.Message, &vm, proto.UnmarshalOptions{}); err != nil {
		return err
	}
	gcpKey, err := v.Cluster().AuthKey("gcp")
	if err != nil {
		return fmt.Errorf("gcp auth key: %w", err)
	}
	gcp := util.GcpApi{ServiceAccount: gcpKey.Gcp.Iam, ProjectId: gcpKey.Gcp.ProjectId, Zone: gcpKey.Gcp.Zone}
	if err := gcp.Auth(); err != nil {
		return fmt.Errorf("gcp auth: %w", err)
	}
	defer gcp.Close()

	for i := uint32(1); i <= vm.NumberOfInstances; i++ {
		name := fmt.Sprintf("%s-%02d", gcpKey.Gcp.Prefix, i)
		core.AppLog.Info().Msgf("creating instance %s", name)
		if err := gcp.Insert(name, gcpKey.Gcp.MachineType, gcpKey.Gcp.ImageType); err != nil {
			return fmt.Errorf("create instance %s: %w", name, err)
		}
		core.AppLog.Info().Msgf("instance %s created", name)
	}
	return v.insert(t.Meta)
}

func (v *VMObjectCreate) confirm(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create confirm %v", t.Meta)
	return v.insert(t.Meta)
}

func (v *VMObjectCreate) cancel(t *protocol.Transaction) error {
	core.AppLog.Debug().Msgf("create cancel %v", t.Meta)
	var vm protocol.VMObject
	if err := anypb.UnmarshalTo(t.Message, &vm, proto.UnmarshalOptions{}); err != nil {
		core.AppLog.Warn().Msgf("cancel unmarshal: %s", err)
		return v.insert(t.Meta)
	}
	gcpKey, err := v.Cluster().AuthKey("gcp")
	if err != nil {
		core.AppLog.Warn().Msgf("cancel gcp auth key: %s", err)
		return v.insert(t.Meta)
	}
	gcp := util.GcpApi{ServiceAccount: gcpKey.Gcp.Iam, ProjectId: gcpKey.Gcp.ProjectId, Zone: gcpKey.Gcp.Zone}
	if err := gcp.Auth(); err != nil {
		core.AppLog.Warn().Msgf("cancel gcp auth: %s", err)
		return v.insert(t.Meta)
	}
	defer gcp.Close()

	for i := uint32(1); i <= vm.NumberOfInstances; i++ {
		name := fmt.Sprintf("%s-%02d", gcpKey.Gcp.Prefix, i)
		core.AppLog.Info().Msgf("deleting instance %s (cancel rollback)", name)
		if err := gcp.Delete(name); err != nil {
			core.AppLog.Warn().Msgf("delete instance %s: %s", name, err)
		}
	}
	return v.insert(t.Meta)
}
