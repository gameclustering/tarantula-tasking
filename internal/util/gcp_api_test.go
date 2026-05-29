package util

import (
	"fmt"
	"testing"
)

func TestGcpApi(t *testing.T) {
	vc := vaultClient(t)
	if err := vc.Auth(); err != nil {
		t.Fatalf("vault auth: %s", err)
	}
	ak, err := vc.Load("dev/presence", "gcp")
	if err != nil {
		t.Fatalf("vault load gcp: %s", err)
	}
	cfg := ak.Gcp
	gcp := GcpApi{ServiceAccount: cfg.Iam, ProjectId: cfg.ProjectId, Zone: cfg.Zone}
	if err := gcp.Auth(); err != nil {
		t.Fatalf("gcp auth: %s", err)
	}
	instanceName := fmt.Sprintf("%s-%d", cfg.Prefix, 1)
	if err := gcp.Insert(instanceName, cfg.MachineType, cfg.ImageType); err != nil {
		t.Fatalf("gcp insert: %s", err)
	}
	if err := gcp.Delete(instanceName); err != nil {
		t.Fatalf("gcp delete: %s", err)
	}
}
