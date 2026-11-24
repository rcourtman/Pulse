package configapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// An archive is its submitter's input: a tenant administrator's import is held
// to the addresses that administrator may enter on the settings routes, in
// whichever section of the archive they appear and however they are spelled.
func TestCheckTenantImportHoldsArchiveDestinationsToTenantOutboundPolicy(t *testing.T) {
	instance := context.Background()
	selfHosted := securityutil.WithTenantOutboundPolicy(context.Background(), securityutil.TenantOutboundPolicy{AllowPrivateNetworks: true})
	cloud := securityutil.WithTenantOutboundPolicy(context.Background(), securityutil.TenantOutboundPolicy{})

	pve := func(host string, members ...config.ClusterEndpoint) config.ExportData {
		return config.ExportData{Nodes: config.NodesConfig{PVEInstances: []config.PVEInstance{{Name: "pve", Host: host, IsCluster: len(members) > 0, ClusterEndpoints: members}}}}
	}
	pbs := func(host string) config.ExportData {
		return config.ExportData{Nodes: config.NodesConfig{PBSInstances: []config.PBSInstance{{Name: "pbs", Host: host}}}}
	}
	pmg := func(host string) config.ExportData {
		return config.ExportData{Nodes: config.NodesConfig{PMGInstances: []config.PMGInstance{{Name: "pmg", Host: host}}}}
	}
	smtp := func(host string) config.ExportData {
		return config.ExportData{Email: notifications.EmailConfig{SMTPHost: host}}
	}
	const allowed = "https://192.168.1.20:8006"
	// A label longer than 63 bytes is not a DNS name, so the resolver reports
	// it as not found without asking a server: a member name with no record.
	unresolvedMember := "https://" + strings.Repeat("n", 64) + ":8006"

	for _, tc := range []struct {
		name    string
		ctx     context.Context
		archive config.ExportData
		refused bool
	}{
		{"instance administrator, loopback everywhere", instance, pve("https://127.0.0.1:8006", config.ClusterEndpoint{Host: "https://127.0.0.1:8006", IP: "127.0.0.1", IPOverride: "169.254.169.254"}), false},
		{"instance administrator, loopback SMTP", instance, smtp("127.0.0.1"), false},

		{"PVE on loopback", selfHosted, pve("https://127.0.0.1:8006"), true},
		{"PVE on loopback without scheme or port", selfHosted, pve("127.0.0.1"), true},
		{"PVE on IPv6 loopback", selfHosted, pve("https://[::1]:8006"), true},
		{"PVE host that only reads as another address", selfHosted, pve("localhost://192.168.1.20:8006"), true},
		{"PVE behind userinfo", selfHosted, pve("https://192.168.1.20:8006@127.0.0.1:8006"), true},
		{"PBS on loopback", selfHosted, pbs("http://127.0.0.1:9922"), true},
		{"PBS on cloud metadata", selfHosted, pbs("http://169.254.169.254"), true},
		{"PMG on loopback", selfHosted, pmg("https://127.0.0.1:8006"), true},
		{"SMTP on loopback", selfHosted, smtp("127.0.0.1"), true},
		{"cluster member host on loopback", selfHosted, pve(allowed, config.ClusterEndpoint{NodeName: "pve2", Host: "http://127.0.0.1:9922"}), true},
		{"cluster member IP on loopback", selfHosted, pve(allowed, config.ClusterEndpoint{NodeName: "pve2", IP: "127.0.0.1"}), true},
		{"cluster member IP as a URL", selfHosted, pve(allowed, config.ClusterEndpoint{NodeName: "pve2", IP: "http://192.168.1.21:8006"}), true},
		{"cluster member IP behind userinfo", selfHosted, pve(allowed, config.ClusterEndpoint{NodeName: "pve2", IP: "192.168.1.21:80@127.0.0.1"}), true},
		{"cluster member override on link-local", selfHosted, pve(allowed, config.ClusterEndpoint{NodeName: "pve2", IP: "192.168.1.21", IPOverride: "169.254.169.254:80"}), true},
		{"private address on Pulse Cloud", cloud, pve(allowed), true},
		{"private cluster member on Pulse Cloud", cloud, pve("https://93.184.216.34:8006", config.ClusterEndpoint{NodeName: "pve2", IP: "10.0.0.5"}), true},

		{"nothing to connect to", selfHosted, config.ExportData{}, false},
		{"private addresses on a self-hosted install", selfHosted, pve(allowed, config.ClusterEndpoint{NodeName: "pve2", Host: "https://192.168.1.21:8006", IP: "192.168.1.21", IPOverride: "10.0.0.5:8006"}), false},
		{"PBS, PMG and SMTP on private addresses", selfHosted, config.ExportData{
			Nodes: config.NodesConfig{PBSInstances: []config.PBSInstance{{Name: "pbs", Host: "192.168.1.30"}}, PMGInstances: []config.PMGInstance{{Name: "pmg", Host: "https://192.168.1.31:8006"}}},
			Email: notifications.EmailConfig{SMTPHost: "192.168.1.32"},
		}, false},
		{"cluster member name without a DNS record", selfHosted, pve(allowed, config.ClusterEndpoint{NodeName: "pve2", Host: unresolvedMember, IP: "192.168.1.21"}), false},
		{"public address on Pulse Cloud", cloud, pve("https://93.184.216.34:8006"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkTenantImport(tc.ctx, &tc.archive, config.NewConfigPersistence(t.TempDir()))
			if !tc.refused {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			var refusal *importRefusal
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want an import refusal", err)
			}
			if !errors.Is(err, securityutil.ErrTenantOutboundDestination) {
				t.Fatalf("refusal %v does not wrap ErrTenantOutboundDestination", err)
			}
		})
	}
}

// The tolerance for a cluster member's node name is for that name only. An
// address an administrator enters is refused when it does not resolve, as it
// is on the settings routes.
func TestCheckTenantImportRefusesEnteredNamesThatDoNotResolve(t *testing.T) {
	ctx := securityutil.WithTenantOutboundPolicy(context.Background(), securityutil.TenantOutboundPolicy{AllowPrivateNetworks: true})
	unresolved := strings.Repeat("n", 64)
	for name, archive := range map[string]config.ExportData{
		"PVE host": {Nodes: config.NodesConfig{PVEInstances: []config.PVEInstance{{Name: "pve", Host: "https://" + unresolved + ":8006"}}}},
		"PBS host": {Nodes: config.NodesConfig{PBSInstances: []config.PBSInstance{{Name: "pbs", Host: "https://" + unresolved + ":8007"}}}},
		"cluster member override": {Nodes: config.NodesConfig{PVEInstances: []config.PVEInstance{{Name: "pve", Host: "https://192.168.1.20:8006",
			ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "pve2", IPOverride: unresolved}}}}}},
		"SMTP server": {Email: notifications.EmailConfig{SMTPHost: unresolved}},
	} {
		if err := checkTenantImport(ctx, &archive, config.NewConfigPersistence(t.TempDir())); err == nil {
			t.Errorf("%s that does not resolve was admitted", name)
		}
	}
}

// Apprise command-line targets run as the server, so a tenant administrator's
// archive cannot introduce them. Restoring the targets an instance
// administrator already saved adds nothing and stays possible.
func TestCheckTenantImportKeepsAppriseCommandLineTargetsWithInstanceAdministrators(t *testing.T) {
	tenant := securityutil.WithTenantOutboundPolicy(context.Background(), securityutil.TenantOutboundPolicy{AllowPrivateNetworks: true})
	cli := notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeCLI, Targets: []string{"json://127.0.0.1:9922/chosen-by-tenant"}}
	api := notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://apprise.example.com", Targets: cli.Targets}

	for _, tc := range []struct {
		name    string
		ctx     context.Context
		saved   *notifications.AppriseConfig
		archive notifications.AppriseConfig
		refused bool
	}{
		{"instance administrator", context.Background(), nil, cli, false},
		{"new command-line targets", tenant, nil, cli, true},
		{"command-line mode by default", tenant, nil, notifications.AppriseConfig{Targets: cli.Targets}, true},
		{"other command-line targets than saved", tenant, &notifications.AppriseConfig{Mode: notifications.AppriseModeCLI, Targets: []string{"mailto://ops@example.com"}}, cli, true},
		{"targets saved for an API server", tenant, &api, cli, true},
		{"command-line targets already saved", tenant, &cli, cli, false},
		{"API server", tenant, nil, api, false},
		{"command line without targets", tenant, nil, notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeCLI}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := config.NewConfigPersistence(t.TempDir())
			if tc.saved != nil {
				if err := saved.SaveAppriseConfig(*tc.saved); err != nil {
					t.Fatal(err)
				}
			}
			err := checkTenantImport(tc.ctx, &config.ExportData{Apprise: tc.archive}, saved)
			var refusal *importRefusal
			if tc.refused != errors.As(err, &refusal) {
				t.Fatalf("err = %v, want refusal = %v", err, tc.refused)
			}
		})
	}
}
