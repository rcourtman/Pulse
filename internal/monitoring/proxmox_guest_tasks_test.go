package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type guestTaskPVEClient struct {
	*stubPVEClient
	window proxmox.GuestTaskWindow
	err    error
	node   string
	vmid   int
	since  time.Time
	until  time.Time
}

func (c *guestTaskPVEClient) GetGuestTasks(_ context.Context, node string, vmid int, since, until time.Time) (proxmox.GuestTaskWindow, error) {
	c.node, c.vmid, c.since, c.until = node, vmid, since, until
	return c.window, c.err
}

func TestReadProxmoxGuestTasksKeepsWhetherTheReadWasWhole(t *testing.T) {
	since, until := time.Unix(1700000000, 0).UTC(), time.Unix(1700000600, 0).UTC()
	client := &guestTaskPVEClient{
		stubPVEClient: &stubPVEClient{},
		window: proxmox.GuestTaskWindow{
			Complete: true,
			Tasks: []proxmox.Task{
				{Type: "vzstop", ID: "110", User: "root@pam", Status: "OK", Node: "node-a", StartTime: 1700000100, EndTime: 1700000102},
				{Type: "vzshutdown", ID: "110", User: "ops@pve", Node: "node-a", StartTime: 1700000200},
			},
		},
	}
	monitor := &Monitor{pveClients: map[string]PVEClientInterface{"homelab": client}}

	read := monitor.ReadProxmoxGuestTasks(context.Background(), " homelab ", " node-a ", 110, since, until)
	if !read.Complete || read.Reason != "" || len(read.Tasks) != 2 {
		t.Fatalf("read = %+v, want two tasks from a whole read", read)
	}
	if client.node != "node-a" || client.vmid != 110 || !client.since.Equal(since) || !client.until.Equal(until) {
		t.Fatalf("client asked for %q %d %s..%s", client.node, client.vmid, client.since, client.until)
	}
	done, running := read.Tasks[0], read.Tasks[1]
	if done.Type != "vzstop" || done.User != "root@pam" || done.Status != "OK" || !done.StartedAt.Equal(time.Unix(1700000100, 0)) || !done.EndedAt.Equal(time.Unix(1700000102, 0)) {
		t.Fatalf("finished task = %+v", done)
	}
	if running.Type != "vzshutdown" || !running.EndedAt.IsZero() {
		t.Fatalf("running task = %+v, want no end time", running)
	}

	// A window that is not the whole of it stays marked, with its reason.
	client.window = proxmox.GuestTaskWindow{Reason: proxmox.GuestTasksAuditUnconfirmed}
	if read := monitor.ReadProxmoxGuestTasks(context.Background(), "homelab", "node-a", 110, since, until); read.Complete || read.Reason != proxmox.GuestTasksAuditUnconfirmed {
		t.Fatalf("partial read = %+v", read)
	}

	// A failed request is never a whole read, even when the client says so.
	client.window, client.err = proxmox.GuestTaskWindow{Complete: true}, errors.New("boom")
	if read := monitor.ReadProxmoxGuestTasks(context.Background(), "homelab", "node-a", 110, since, until); read.Complete || read.Reason != proxmox.GuestTasksCollectionFailed {
		t.Fatalf("failed read = %+v", read)
	}
}

func TestReadProxmoxGuestTasksWithoutAConnectionIsNotAnEmptyLog(t *testing.T) {
	since, until := time.Unix(1700000000, 0), time.Unix(1700000600, 0)
	var absent *Monitor
	monitors := map[string]*Monitor{
		"no monitor":          absent,
		"no client":           {pveClients: map[string]PVEClientInterface{}},
		"client cannot list":  {pveClients: map[string]PVEClientInterface{"homelab": &stubPVEClient{}}},
		"missing guest or id": {pveClients: map[string]PVEClientInterface{"homelab": &guestTaskPVEClient{stubPVEClient: &stubPVEClient{}, window: proxmox.GuestTaskWindow{Complete: true}}}},
	}
	for name, monitor := range monitors {
		vmid := 110
		if name == "missing guest or id" {
			vmid = 0
		}
		read := monitor.ReadProxmoxGuestTasks(context.Background(), "homelab", "node-a", vmid, since, until)
		if read.Complete || read.Reason != ProxmoxGuestTasksNoConnection || len(read.Tasks) != 0 {
			t.Fatalf("%s: read = %+v, want an unread log", name, read)
		}
	}
}
