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

	"github.com/stretchr/testify/require"
)

func TestTaskLogRejectsUnverifiedResponse(t *testing.T) {
	const row = `{"n":1,"t":"INFO: Starting Backup of VM 101 (qemu)"}`
	const response = `{"data":[` + row + `]}`
	for _, tc := range []struct {
		name, body string
		unfinished bool
	}{
		{"missing-data", `{}`, false},
		{"null-data", `{"data":null}`, false},
		{"null-envelope", `null`, false},
		{"not-array", `{"data":{}}`, false},
		{"error-with-data", `{"data":[],"errors":{"message":"fixture-private-detail"}}`, false},
		{"case-data", `{"Data":[]}`, false},
		{"duplicate-data", `{"data":[` + row + `],"data":[]}`, false},
		{"escaped-duplicate-data", `{"data":[],"\u0064ata":[]}`, false},
		{"case-overwrite", `{"data":[` + row + `],"Data":[]}`, false},
		{"case-error", `{"data":[],"Errors":null}`, false},
		{"trailing-value", response + ` {"data":[]}`, false},
		{"trailing-text", response + `fixture-private-detail`, false},
		{"unfinished-body", response, true},
		{"oversized-body", response + strings.Repeat(" ", int(maxResponseBodyBytes)), false},
		{"invalid-utf8", `{"data":[` + row + `],"metadata":"` + string([]byte{0xff}) + `"}`, false},
		{"null-row", `{"data":[` + row + `,null]}`, false},
		{"missing-number", `{"data":[{"t":"fixture-private-detail"}]}`, false},
		{"null-number", `{"data":[{"n":null,"t":"text"}]}`, false},
		{"string-number", `{"data":[{"n":"1","t":"text"}]}`, false},
		{"fractional-number", `{"data":[{"n":1.5,"t":"text"}]}`, false},
		{"zero-number", `{"data":[{"n":0,"t":"text"}]}`, false},
		{"negative-number", `{"data":[{"n":-1,"t":"text"}]}`, false},
		{"missing-first-line", `{"data":[{"n":2,"t":"text"}]}`, false},
		{"missing-text", `{"data":[{"n":1}]}`, false},
		{"null-text", `{"data":[{"n":1,"t":null}]}`, false},
		{"non-string-text", `{"data":[{"n":1,"t":42}]}`, false},
		{"duplicate-number", `{"data":[{"n":2,"n":1,"t":"text"}]}`, false},
		{"duplicate-text", `{"data":[{"n":1,"t":"first","\u0074":"second"}]}`, false},
		{"case-number", `{"data":[{"n":1,"N":2,"t":"text"}]}`, false},
		{"case-text", `{"data":[{"n":1,"t":"first","T":"second"}]}`, false},
		{"repeated-line", `{"data":[` + row + `,` + row + `]}`, false},
		{"gap-after-good-line", `{"data":[` + row + `,{"n":3,"t":"INFO: Finished Backup of VM 101 (00:01:00)"}]}`, false},
		{"reordered-lines", `{"data":[{"n":2,"t":"INFO: Finished Backup of VM 101 (00:01:00)"},` + row + `]}`, false},
		{"invalid-later-row", `{"data":[` + row + `,{"n":2,"t":false}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var reads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reads.Add(1)
				require.Equal(t, "/api2/json/nodes/member/tasks/job/log", r.URL.Path)
				require.Equal(t, "0", r.URL.Query().Get("start"))
				require.Equal(t, fmt.Sprint(taskLogLineLimit), r.URL.Query().Get("limit"))
				if tc.unfinished {
					w.Header().Set("Content-Length", fmt.Sprint(len(tc.body)+100))
				} else {
					w.(http.Flusher).Flush() // whole-body bound, not a Content-Length shortcut
				}
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			lines, err := backupTestClient(t, server.URL).GetTaskLog(context.Background(), "member", "job")
			require.Error(t, err)
			require.Nil(t, lines, "a good prefix cannot publish partial guest evidence")
			require.NotContains(t, err.Error(), "fixture-private-detail")
			require.EqualValues(t, 1, reads.Load(), "invalid successful reads are not retried")
			_, typed := APIErrorStatus(err)
			require.False(t, typed, "malformed success is not a permission denial")
		})
	}
}

func TestTaskLogAcceptsExplicitPrefixAndExtensions(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []TaskLogLine
	}{
		{"empty", `{"data":[]}`, []TaskLogLine{}},
		{"empty-with-metadata", `{"data":[],"errors":null,"total":0}`, []TaskLogLine{}},
		{"blank-and-unicode", `{"data":[{"n":1,"t":"","provider-metadata":true},{"n":2,"t":"INFO: café 日本語"}],"errors":null}`, []TaskLogLine{{1, ""}, {2, "INFO: café 日本語"}}},
		{"running-prefix", `{"data":[{"n":1,"t":"INFO: Starting Backup of VM 101 (qemu)"}]}`, []TaskLogLine{{1, "INFO: Starting Backup of VM 101 (qemu)"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
			defer server.Close()
			lines, err := backupTestClient(t, server.URL).GetTaskLog(context.Background(), "member", "job")
			require.NoError(t, err)
			require.Equal(t, tc.want, lines)
		})
	}
	for _, count := range []int{taskLogLineLimit, taskLogLineLimit + 1} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			rows := make([]TaskLogLine, count)
			for i := range rows {
				rows[i] = TaskLogLine{LineNumber: int64(i + 1), Text: "INFO: progress"}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(map[string]any{"data": rows}) }))
			defer server.Close()
			lines, err := backupTestClient(t, server.URL).GetTaskLog(context.Background(), "member", "job")
			if count == taskLogLineLimit {
				require.NoError(t, err)
				require.Equal(t, rows, lines, "the existing capped prefix stays usable, not complete")
			} else {
				require.Error(t, err)
				require.Nil(t, lines, "never silently truncate an over-limit reply")
			}
		})
	}
}

func TestTaskLogReadClosureAndStatus(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusPartialContent, http.StatusInternalServerError} {
		lines, err := readTaskLogResponse(&http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))})
		require.Error(t, err)
		require.Nil(t, lines)
	}
	body := &snapshotCloseFailure{Reader: strings.NewReader(`{"data":[{"n":1,"t":"accepted prefix"}]}`)}
	lines, err := readTaskLogResponse(&http.Response{StatusCode: http.StatusOK, Body: body})
	require.Error(t, err)
	require.Nil(t, lines)
	require.True(t, body.closed)
}
