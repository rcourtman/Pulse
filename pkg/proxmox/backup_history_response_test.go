package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const acceptedHistoryRow = `{"upid":"accepted","node":"member","type":"vzdump","id":"101","starttime":1700000000,"endtime":1700000060,"status":"OK"}`

func TestBackupHistoryRejectsUnverifiedTaskInventory(t *testing.T) {
	valid := `{"data":[` + acceptedHistoryRow + `]}`
	for _, tc := range []struct {
		name, body string
		incomplete bool
	}{
		{"missing-data", `{}`, false},
		{"null-data", `{"data":null}`, false},
		{"null-envelope", `null`, false},
		{"non-array", `{"data":{}}`, false},
		{"error-as-success", `{"data":[],"errors":{"message":"fixture-private-detail"}}`, false},
		{"case-data", `{"Data":[]}`, false},
		{"duplicate-data", `{"data":[` + acceptedHistoryRow + `],"data":[]}`, false},
		{"escaped-duplicate", `{"data":[],"\u0064ata":[]}`, false},
		{"case-overwrite", `{"data":[` + acceptedHistoryRow + `],"Data":[]}`, false},
		{"case-errors", `{"data":[],"Errors":null}`, false},
		{"trailing-value", valid + ` {"data":[]}`, false},
		{"trailing-text", valid + `fixture-private-detail`, false},
		{"unfinished-response", valid, true},
		{"oversized-response", valid + strings.Repeat(" ", int(maxResponseBodyBytes)), false},
		{"invalid-utf8", `{"data":[],"metadata":"` + string([]byte{0xff}) + `"}`, false},
		{"null-row", `{"data":[` + acceptedHistoryRow + `,null]}`, false},
		{"duplicate-row", `{"data":[` + acceptedHistoryRow + `,` + acceptedHistoryRow + `]}`, false},
		{"missing-upid", `{"data":[{"type":"vzdump","starttime":1700000000}]}`, false},
		{"blank-upid", `{"data":[{"upid":" ","type":"vzdump","starttime":1700000000}]}`, false},
		{"duplicate-upid-field", `{"data":[{"upid":"first",` + acceptedHistoryRow[1:] + `]}`, false},
		{"case-status-field", `{"data":[{"Status":"running",` + acceptedHistoryRow[1:] + `]}`, false},
		{"missing-type", `{"data":[{"upid":"accepted","starttime":1700000000}]}`, false},
		{"missing-start", `{"data":[{"upid":"accepted","type":"vzdump"}]}`, false},
		{"null-start", `{"data":[{"upid":"accepted","type":"vzdump","starttime":null}]}`, false},
		{"negative-start", `{"data":[{"upid":"accepted","type":"vzdump","starttime":-1}]}`, false},
		{"reversed-time", `{"data":[{"upid":"accepted","type":"vzdump","starttime":1700000000,"endtime":1600000000}]}`, false},
		{"null-end", `{"data":[{"upid":"accepted","type":"vzdump","starttime":1700000000,"endtime":null}]}`, false},
		{"null-status", `{"data":[{"upid":"accepted","type":"vzdump","starttime":1700000000,"status":null}]}`, false},
		{"invalid-guest", `{"data":[{"upid":"accepted","type":"vzdump","starttime":1700000000,"id":"not-a-guest"}]}`, false},
		{"foreign-node", `{"data":[{"upid":"accepted","node":"other","type":"vzdump","starttime":1700000000}]}`, false},
		{"bad-row-after-good", `{"data":[` + acceptedHistoryRow + `,{"upid":"second","type":false}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var taskReads, auditReads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
				case "/api2/json/nodes/member/tasks":
					taskReads.Add(1)
					if tc.incomplete {
						w.Header().Set("Content-Length", fmt.Sprint(len(tc.body)+100))
					}
					fmt.Fprint(w, tc.body)
				case "/api2/json/access/permissions":
					auditReads.Add(1)
					fmt.Fprint(w, `{"data":{"/nodes/member":{"Sys.Audit":1}}}`)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			history, err := backupTestClient(t, server.URL).GetBackupTaskHistory(context.Background(), time.Unix(1600000000, 0))
			require.NoError(t, err, "per-node failures remain explicit alongside independent evidence")
			require.Empty(t, history.Tasks, "no prefix or partially valid page may publish tasks")
			require.Equal(t, []BackupTaskHistoryNode{{Node: "member", Reason: "collection_failed"}}, history.Nodes)
			require.EqualValues(t, 1, taskReads.Load(), "invalid successful responses are not retried")
			require.Zero(t, auditReads.Load(), "an unread inventory cannot qualify by checking permissions")
		})
	}
}

func TestBackupHistoryRequiresCompleteAuditEvidence(t *testing.T) {
	valid := `{"data":{"/nodes/member":{"Sys.Audit":1}}}`
	for _, tc := range []struct {
		name, body string
		incomplete bool
	}{
		{"missing", `{}`, false},
		{"null", `{"data":null}`, false},
		{"trailing", valid + ` {}`, false},
		{"unfinished", valid, true},
		{"duplicate-data", `{"data":{},"data":{"/nodes/member":{"Sys.Audit":1}}}`, false},
		{"case-data", `{"Data":{"/nodes/member":{"Sys.Audit":1}}}`, false},
		{"duplicate-node", `{"data":{"/nodes/member":{},"/nodes/member":{"Sys.Audit":1}}}`, false},
		{"duplicate-grant", `{"data":{"/nodes/member":{"Sys.Audit":0,"Sys.Audit":1}}}`, false},
		{"case-grant", `{"data":{"/nodes/member":{"Sys.Audit":1,"sys.audit":0}}}`, false},
		{"contradictory-error", `{"data":{"/nodes/member":{"Sys.Audit":1}},"errors":{"message":"fixture-private-detail"}}`, false},
		{"other-path", `{"data":{"/nodes/other":{"Sys.Audit":1}}}`, false},
		{"not-granted", `{"data":{"/nodes/member":{"Sys.Audit":0}}}`, false},
		{"oversized", valid + strings.Repeat(" ", int(maxResponseBodyBytes)), false},
		{"invalid-utf8", `{"data":{"/nodes/member":{"Sys.Audit":1}},"metadata":"` + string([]byte{0xff}) + `"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
				case "/api2/json/nodes/member/tasks":
					fmt.Fprint(w, `{"data":[`+acceptedHistoryRow+`]}`)
				case "/api2/json/access/permissions":
					if tc.incomplete {
						w.Header().Set("Content-Length", fmt.Sprint(len(tc.body)+100))
					}
					fmt.Fprint(w, tc.body)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			history, err := backupTestClient(t, server.URL).GetBackupTaskHistory(context.Background(), time.Unix(1600000000, 0))
			require.NoError(t, err)
			require.Len(t, history.Tasks, 1, "unconfirmed audit does not erase accepted task evidence")
			require.Equal(t, []BackupTaskHistoryNode{{Node: "member", Reason: "audit_permission_unconfirmed"}}, history.Nodes)
		})
	}
}

func TestBackupHistoryAcceptsExplicitInventoryAndScopedAuthority(t *testing.T) {
	for _, body := range []string{`{"data":[]}`, `{"data":[],"errors":null,"total":0}`, `{"data":[` + acceptedHistoryRow + `],"provider-metadata":{"revision":1}}`, `{"data":[{"upid":"running","type":"vzdump","id":"","starttime":1700000000}]}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
				case "/api2/json/nodes/member/tasks":
					fmt.Fprint(w, body)
				case "/api2/json/access/permissions":
					require.Equal(t, "/nodes/member", r.URL.Query().Get("path"))
					fmt.Fprint(w, `{"data":{"/nodes/member":{"Sys.Audit":1,"VM.Audit":1},"/":{"Other":1}},"errors":null}`)
				}
			}))
			defer server.Close()
			history, err := backupTestClient(t, server.URL).GetBackupTaskHistory(context.Background(), time.Unix(1600000000, 0))
			require.NoError(t, err)
			require.Equal(t, []BackupTaskHistoryNode{{Node: "member", Complete: true}}, history.Nodes)
			if strings.Contains(body, "upid") {
				require.Len(t, history.Tasks, 1)
				require.Equal(t, "member", history.Tasks[0].Node)
			} else {
				require.Empty(t, history.Tasks)
			}
		})
	}
}

func TestBackupHistoryReadClosureAndStatus(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusPartialContent} {
		resp := &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}
		_, err := readBackupTaskInventory(resp, "member")
		require.Error(t, err)
	}
	body := &snapshotCloseFailure{Reader: strings.NewReader(`{"data":[]}`)}
	_, err := readBackupTaskInventory(&http.Response{StatusCode: http.StatusOK, Body: body}, "member")
	require.Error(t, err)
	require.True(t, body.closed)
	for _, resp := range []*http.Response{
		{StatusCode: http.StatusPartialContent, Body: io.NopCloser(strings.NewReader(`{"data":{"/nodes/member":{"Sys.Audit":1}}}`))},
		{StatusCode: http.StatusOK, Body: &snapshotCloseFailure{Reader: strings.NewReader(`{"data":{"/nodes/member":{"Sys.Audit":1}}}`)}},
	} {
		require.False(t, readBackupHistoryAuditPermission(resp, "/nodes/member"))
	}
}

func TestBackupHistoryMalformedLaterPageDoesNotQualifyPrefix(t *testing.T) {
	for _, second := range []string{`{}`, `{"data":null}`, `{"data":[{"upid":"invalid","type":"vzdump"}]}`} {
		t.Run(second, func(t *testing.T) {
			var auditReads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"member","status":"online"}]}`)
				case "/api2/json/nodes/member/tasks":
					if r.URL.Query().Get("start") != "0" {
						fmt.Fprint(w, second)
						return
					}
					tasks := make([]Task, 200)
					for i := range tasks {
						tasks[i] = Task{UPID: fmt.Sprint(i), Type: "vzdump", ID: "101", StartTime: 1700000000}
					}
					json.NewEncoder(w).Encode(map[string]any{"data": tasks})
				case "/api2/json/access/permissions":
					auditReads.Add(1)
					fmt.Fprint(w, `{"data":{"/nodes/member":{"Sys.Audit":1}}}`)
				}
			}))
			defer server.Close()
			history, err := backupTestClient(t, server.URL).GetBackupTaskHistory(context.Background(), time.Unix(1600000000, 0))
			require.NoError(t, err)
			require.Len(t, history.Tasks, 200, "accepted independent pages remain evidence, never completeness")
			require.Equal(t, []BackupTaskHistoryNode{{Node: "member", Reason: "collection_failed"}}, history.Nodes)
			require.Zero(t, auditReads.Load())
		})
	}
}
