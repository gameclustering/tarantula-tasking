package util

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/auth/credentials"
	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
)

type OnInstance func(*computepb.Instance)

type GcpApi struct {
	ServiceAccount string
	ProjectId      string
	Zone           string
	client         *compute.InstancesClient
}

func (g *GcpApi) Auth() error {
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes:          []string{"https://www.googleapis.com/auth/compute"},
		CredentialsJSON: []byte(g.ServiceAccount),
	})
	if err != nil {
		return err
	}
	client, err := compute.NewInstancesRESTClient(context.Background(), option.WithAuthCredentials(creds))
	if err != nil {
		return err
	}
	g.client = client
	return nil
}

func (g *GcpApi) Close() error {
	return g.client.Close()
}

func (g *GcpApi) List(ins OnInstance) error {
	req := &computepb.ListInstancesRequest{
		Project: g.ProjectId,
		Zone:    g.Zone,
	}
	it := g.client.List(context.Background(), req)
	for {
		instance, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("list instances: %w", err)
		}
		ins(instance)
	}
	return nil
}

func (g *GcpApi) Get(name string) (*computepb.Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req := &computepb.GetInstanceRequest{
		Project:  g.ProjectId,
		Zone:     g.Zone,
		Instance: name,
	}
	return g.client.Get(ctx, req)
}

func (g *GcpApi) Insert(name, machineType, imageType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req := &computepb.InsertInstanceRequest{
		Project: g.ProjectId,
		Zone:    g.Zone,
		InstanceResource: &computepb.Instance{
			Name:        proto.String(name),
			MachineType: proto.String(fmt.Sprintf("zones/%s/machineTypes/%s", g.Zone, machineType)),
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					Name: proto.String("global/networks/default"),
					AccessConfigs: []*computepb.AccessConfig{
						{
							Type:        proto.String("ONE_TO_ONE_NAT"),
							Name:        proto.String("External NAT"),
							NetworkTier: proto.String("STANDARD"),
						},
					},
				},
			},
			Tags: &computepb.Tags{
				Items: []string{"http-server", "https-server"},
			},
			Disks: []*computepb.AttachedDisk{
				{
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						SourceImage: proto.String(imageType),
						DiskSizeGb:  proto.Int64(10),
					},
					AutoDelete: proto.Bool(true),
					Boot:       proto.Bool(true),
					Type:       proto.String(computepb.AttachedDisk_PERSISTENT.String()),
				},
			},
		},
	}
	opt, err := g.client.Insert(ctx, req)
	if err != nil {
		return err
	}
	return opt.Wait(ctx)
}

func (g *GcpApi) Delete(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req := &computepb.DeleteInstanceRequest{
		Project:  g.ProjectId,
		Zone:     g.Zone,
		Instance: name,
	}
	opt, err := g.client.Delete(ctx, req)
	if err != nil {
		return err
	}
	return opt.Wait(ctx)
}
