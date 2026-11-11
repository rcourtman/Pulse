package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A node answers 200 with only the caller's own tasks when the caller lacks
// Sys.Audit on it. So an empty window is "nothing happened" only when the
// audit permission is confirmed, the request was not cut off, and it worked.
func TestGetGuestTasksSaysWhetherTheWindowIsTheWholeOfIt(t *testing.T) {
	stop := `{"upid":"UPID:member:1","node":"member","type":"vzstop","id":"110","user":"root@pam","starttime":1700000100,"endtime":1700000102,"status":"OK"}`
	other := `{"upid":"UPID:member:2","node":"member","type":"vzstop","id":"111","user":"root@pam","starttime":1700000100,"endtime":1700000102,"status":"OK"}`
	full := make([]string, 0, guestTaskPageLimit)
	for i := 0; i < guestTaskPageLimit; i++ {
		full = append(full, fmt.Sprintf(`{"upid":"UPID:member:%d","node":"member","type":"vncproxy","id":"110","user":"root@pam","starttime":1700000100,"endtime":1700000102,"status":"OK"}`, 100+i))
	}
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		audit      string
		wantTasks  int
		wantFull   bool
		wantReason string
		wantErr    bool
	}{
		{name: "stop task, auditor", status: 200, body: `{"data":[` + stop + `]}`, audit: `{"data":{"/nodes/member":{"Sys.Audit":1}}}`, wantTasks: 1, wantFull: true},
		{name: "nothing, auditor", status: 200, body: `{"data":[]}`, audit: `{"data":{"/nodes/member":{"Sys.Audit":1}}}`, wantFull: true},
		{name: "nothing, not an auditor", status: 200, body: `{"data":[]}`, audit: `{"data":{"/nodes/member":{"Sys.Audit":0}}}`, wantReason: GuestTasksAuditUnconfirmed},
		{name: "nothing, permission unreadable", status: 200, body: `{"data":[]}`, audit: `{"data":{}}`, wantReason: GuestTasksAuditUnconfirmed},
		{name: "another guest's row is dropped", status: 200, body: `{"data":[` + stop + `,` + other + `]}`, audit: `{"data":{"/nodes/member":{"Sys.Audit":1}}}`, wantTasks: 1, wantFull: true},
		{name: "a full page is cut off", status: 200, body: `{"data":[` + strings.Join(full, ",") + `]}`, audit: `{"data":{"/nodes/member":{"Sys.Audit":1}}}`, wantTasks: guestTaskPageLimit, wantReason: GuestTasksTruncated},
		{name: "forbidden", status: 403, body: `{"data":null}`, wantReason: GuestTasksAccessDenied, wantErr: true},
		{name: "server error", status: 500, body: `{"data":null}`, wantReason: GuestTasksCollectionFailed, wantErr: true},
		{name: "not a task list", status: 200, body: `{"data":{}}`, wantReason: GuestTasksCollectionFailed, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var query string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes/member/tasks":
					query = r.URL.RawQuery
					w.WriteHeader(tc.status)
					fmt.Fprint(w, tc.body)
				case "/api2/json/access/permissions":
					fmt.Fprint(w, tc.audit)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			window, err := backupTestClient(t, server.URL).GetGuestTasks(context.Background(), "member", 110, time.Unix(1700000000, 0), time.Unix(1700000600, 0))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if len(window.Tasks) != tc.wantTasks || window.Complete != tc.wantFull || window.Reason != tc.wantReason {
				t.Fatalf("window = %d tasks, complete %v, reason %q, want %d, %v, %q", len(window.Tasks), window.Complete, window.Reason, tc.wantTasks, tc.wantFull, tc.wantReason)
			}
			if tc.status == 200 {
				for _, want := range []string{"vmid=110", "source=all", "since=1700000000", "until=1700000600"} {
					if !strings.Contains(query, want) {
						t.Fatalf("query %q lacks %q", query, want)
					}
				}
			}
		})
	}
}

func TestGetGuestTasksNeedsANodeAndAGuest(t *testing.T) {
	client := backupTestClient(t, "http://127.0.0.1:1")
	for _, tc := range []struct {
		node string
		vmid int
	}{{"", 110}, {"member", 0}} {
		window, err := client.GetGuestTasks(context.Background(), tc.node, tc.vmid, time.Unix(1, 0), time.Unix(2, 0))
		if err == nil || window.Complete {
			t.Fatalf("node %q vmid %d: window %+v, err %v, want a refusal", tc.node, tc.vmid, window, err)
		}
	}
}
