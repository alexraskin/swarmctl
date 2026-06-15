package cloudflare

import (
	"context"
	"strconv"
	"testing"

	"github.com/cloudflare/cloudflare-go/v4/zero_trust"
	"github.com/docker/docker/api/types/swarm"
)

// fakeAPI is an in-memory implementation of the API interface for tests.
type fakeAPI struct {
	ingress    []zero_trust.TunnelCloudflaredConfigurationGetResponseConfigIngress
	updates    map[string]string // hostname -> targetURL
	dnsCreated []string
	zoneCalls  int
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{updates: map[string]string{}}
}

func (f *fakeAPI) GetTunnelConfig(ctx context.Context) (*zero_trust.TunnelCloudflaredConfigurationGetResponse, error) {
	return &zero_trust.TunnelCloudflaredConfigurationGetResponse{
		Config: zero_trust.TunnelCloudflaredConfigurationGetResponseConfig{Ingress: f.ingress},
	}, nil
}

func (f *fakeAPI) UpdateTunnelConfig(ctx context.Context, hostname, targetURL string) error {
	f.updates[hostname] = targetURL
	return nil
}

func (f *fakeAPI) GetZoneID(ctx context.Context, hostname string) (string, error) {
	f.zoneCalls++
	return "zone-123", nil
}

func (f *fakeAPI) CreateTunnelDNSRecord(ctx context.Context, zoneID, hostname string) error {
	f.dnsCreated = append(f.dnsCreated, hostname)
	return nil
}

func (f *fakeAPI) DeleteTunnelDNSRecord(ctx context.Context, recordID, zoneID string) error {
	return nil
}

func (f *fakeAPI) GetTunnelDNSRecord(ctx context.Context, zoneID, hostname string) (string, error) {
	return "record-123", nil
}

func newService(name, port string, hostnames ...string) *swarm.Service {
	svc := &swarm.Service{}
	svc.Spec.Name = name
	svc.Spec.Labels = map[string]string{
		"cloudflared.tunnel.enabled": "true",
		"cloudflared.tunnel.port":    port,
	}
	for i, h := range hostnames {
		if i == 0 {
			svc.Spec.Labels["cloudflared.tunnel.hostname"] = h
		} else {
			svc.Spec.Labels["cloudflared.tunnel."+strconv.Itoa(i)+".hostname"] = h
		}
	}
	return svc
}

func TestSyncService_NewHostCreatesDNSAndIngress(t *testing.T) {
	api := newFakeAPI()
	syncer := NewSyncer(api)

	svc := newService("svc", "80", "a.example.com")

	if err := syncer.SyncService(context.Background(), svc); err != nil {
		t.Fatalf("SyncService: %v", err)
	}

	if got := api.updates["a.example.com"]; got != "http://svc:80" {
		t.Fatalf("ingress target = %q, want %q", got, "http://svc:80")
	}
	if len(api.dnsCreated) != 1 || api.dnsCreated[0] != "a.example.com" {
		t.Fatalf("dnsCreated = %v, want [a.example.com]", api.dnsCreated)
	}
}

func TestSyncService_UnchangedHostIsSkipped(t *testing.T) {
	api := newFakeAPI()
	syncer := NewSyncer(api)
	svc := newService("svc", "80", "a.example.com")

	// First sync provisions the host.
	if err := syncer.SyncService(context.Background(), svc); err != nil {
		t.Fatalf("first SyncService: %v", err)
	}

	// Second sync with identical target should not re-create DNS or re-zone.
	api.dnsCreated = nil
	api.zoneCalls = 0
	if err := syncer.SyncService(context.Background(), svc); err != nil {
		t.Fatalf("second SyncService: %v", err)
	}

	if len(api.dnsCreated) != 0 {
		t.Fatalf("dnsCreated on unchanged sync = %v, want none", api.dnsCreated)
	}
	if api.zoneCalls != 0 {
		t.Fatalf("zoneCalls on unchanged sync = %d, want 0", api.zoneCalls)
	}
}

func TestSyncService_InvalidateHostForcesReprovision(t *testing.T) {
	api := newFakeAPI()
	syncer := NewSyncer(api)
	svc := newService("svc", "80", "a.example.com")

	if err := syncer.SyncService(context.Background(), svc); err != nil {
		t.Fatalf("first SyncService: %v", err)
	}

	// Simulate the reconciler removing the host; cache must be invalidated so a
	// recreate re-provisions DNS.
	syncer.InvalidateHost("a.example.com")
	api.dnsCreated = nil

	if err := syncer.SyncService(context.Background(), svc); err != nil {
		t.Fatalf("re-sync after invalidate: %v", err)
	}

	if len(api.dnsCreated) != 1 || api.dnsCreated[0] != "a.example.com" {
		t.Fatalf("dnsCreated after invalidate = %v, want [a.example.com]", api.dnsCreated)
	}
}
