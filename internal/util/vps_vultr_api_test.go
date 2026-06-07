package util

import (
	"fmt"
	"testing"
)

func TestVultrClient(t *testing.T) {
	vc := vaultClient(t)
	if err := vc.Auth(); err != nil {
		t.Fatalf("vault auth: %s", err)
	}
	ak, err := vc.Load("dev/presence", "vps")
	if err != nil {
		t.Fatalf("vault load vps: %s", err)
	}
	if ak.Vps.ApiKey == "" {
		t.Skip("vps apiKey not set")
	}
	va := VultrApi{ApiKey: ak.Vps.ApiKey}
	instances, err := va.ListInstances()
	if err != nil {
		t.Fatalf("list instances: %s", err)
	}
	fmt.Printf("instances %v\n", instances)
}
