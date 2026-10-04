package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuestAgentLockEvidenceRejectsAmbiguity(t *testing.T) {
	cases := []struct{ name, body string }{
		{"duplicate-lock", `{"data":{"lock":"backup","lock":""}}`},
		{"duplicate-unlocked", `{"data":{"lock":"","lock":""}}`},
		{"escaped-lock", `{"data":{"lock":"backup","\u006cock":""}}`},
		{"duplicate-data", `{"data":{"lock":"backup"},"data":{}}`},
		{"escaped-data", `{"data":{"lock":"backup"},"\u0064ata":{}}`},
		{"case-data", `{"data":{"lock":"backup"},"DATA":{}}`},
		{"case-only-data", `{"Data":{}}`},
		{"case-only-lock", `{"data":{"Lock":"backup"}}`},
		{"case-lock", `{"data":{"lock":"","LOCK":"backup"}}`},
		{"competing-error", `{"data":{},"errors":{"message":"lock unavailable"}}`},
		{"competing-failure", `{"data":{},"success":false}`},
		{"duplicate-config-field", `{"data":{"name":"first","name":"second"}}`},
		{"null-lock", `{"data":{"lock": null }}`},
		{"number-lock", `{"data":{"lock":0}}`},
		{"object-lock", `{"data":{"lock":{}}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for name, read := range backupAgentReads() {
				for _, postCommand := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/post=%t", name, postCommand), func(t *testing.T) {
						var configs, commands atomic.Int32
						var healthy atomic.Bool
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if strings.HasSuffix(r.URL.Path, "/config") {
								if configs.Add(1) == 1 && postCommand || healthy.Load() {
									fmt.Fprint(w, `{"data":{"name":"fixture","agent":1}}`)
								} else {
									fmt.Fprint(w, tc.body)
								}
								return
							}
							commands.Add(1)
							backupAgentPayload(w, r)
						}))
						defer server.Close()
						c := backupTestClient(t, server.URL)
						if err := read(context.Background(), c, 105); GuestAgentDeferredReason(err) != "lock-unverified" {
							t.Errorf("ambiguous lock admitted/published a command: %v", err)
						}
						want := int32(0)
						if postCommand {
							want = 1
						}
						if got := commands.Load(); got != want {
							t.Errorf("wire commands = %d, want %d", got, want)
						}
						healthy.Store(true)
						// A completed command's failed lock check is not transport
						// uncertainty. A new diagnostic must reverify and resume.
						diagnostic := backupTestClient(t, server.URL)
						if err := read(context.Background(), diagnostic, 105); err != nil {
							t.Fatalf("complete unlocked config did not resume: %v", err)
						}
						if got := commands.Load(); got != want+1 {
							t.Errorf("resumed wire commands = %d, want %d", got, want+1)
						}
					})
				}
			}
		})
	}
}

func TestGuestAgentLockEvidenceRejectsRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, crossEndpoint := range []bool{false, true} {
			for name, read := range backupAgentReads() {
				for _, postCommand := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d/cross=%t/%s/post=%t", status, crossEndpoint, name, postCommand), func(t *testing.T) {
						var configs, commands, redirected atomic.Int32
						var healthy atomic.Bool
						other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							redirected.Add(1)
							fmt.Fprint(w, `{"data":{}}`)
						}))
						defer other.Close()
						server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							switch {
							case strings.HasSuffix(r.URL.Path, "/config"):
								if configs.Add(1) == 1 && postCommand || healthy.Load() {
									fmt.Fprint(w, `{"data":{}}`)
								} else {
									destination := "/unrelated-unlocked-config"
									if crossEndpoint {
										destination = other.URL + destination
									}
									http.Redirect(w, r, destination, status)
								}
							case r.URL.Path == "/unrelated-unlocked-config":
								redirected.Add(1)
								fmt.Fprint(w, `{"data":{}}`)
							default:
								commands.Add(1)
								backupAgentPayload(w, r)
							}
						}))
						defer server.Close()
						c := backupTestClient(t, server.URL)
						if err := read(context.Background(), c, 105); GuestAgentDeferredReason(err) != "lock-unverified" {
							t.Errorf("redirected lock admitted/published a command: %v", err)
						}
						want := int32(0)
						if postCommand {
							want = 1
						}
						if commands.Load() != want || redirected.Load() != 0 {
							t.Errorf("wire commands/redirected = %d/%d, want %d/0", commands.Load(), redirected.Load(), want)
						}
						// Ordinary config reads retain their existing redirect
						// policy; do not globally mutate c.httpClient.
						if _, err := c.GetVMConfig(context.Background(), "node", 105); err != nil || redirected.Load() != 1 {
							t.Errorf("ordinary config redirect policy changed: redirects=%d err=%v", redirected.Load(), err)
						}
						healthy.Store(true)
						if err := read(context.Background(), backupTestClient(t, server.URL), 105); err != nil {
							t.Fatalf("valid same-target config did not resume: %v", err)
						}
						if commands.Load() != want+1 || redirected.Load() != 1 {
							t.Errorf("resumed commands/redirected = %d/%d, want %d/1", commands.Load(), redirected.Load(), want+1)
						}
					})
				}
			}
		}
	}
}

func TestGuestAgentLockEvidenceHealthyConfigAndSessionRecovery(t *testing.T) {
	for _, body := range []string{
		`{"data":{}}`,
		`{"data":{"lock":"","agent":1,"digest":"fixture","name":"fixture"}}`,
		`{"data":{"lock":"  ","description":"backup; lock","scsi0":"fixture:disk","tags":"backup;ha"}}`,
		`{"data":{"\u006cock":"","nested":{"data":"fixture","lock":"backup"}}}`, // Only the actual operation lock is authoritative.
	} {
		t.Run(fmt.Sprintf("config-%d", len(body)), func(t *testing.T) {
			var configs, commands, authentications atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/access/ticket"):
					authentications.Add(1)
					fmt.Fprint(w, `{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`)
				case strings.HasSuffix(r.URL.Path, "/config"):
					if configs.Add(1) == 1 {
						http.Error(w, "expired config session", http.StatusUnauthorized)
						return
					}
					fmt.Fprint(w, body)
				default:
					commands.Add(1)
					backupAgentPayload(w, r)
				}
			}))
			defer server.Close()
			c, err := NewClient(ClientConfig{Host: server.URL, User: "fixture@pam", Password: "fixture-password", Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if err := backupAgentReads()["filesystem"](context.Background(), c, 105); err != nil {
				t.Fatal(err)
			}
			if configs.Load() != 3 || authentications.Load() != 2 || commands.Load() != 1 {
				t.Errorf("config recovery/command identity lost: configs=%d auth=%d commands=%d", configs.Load(), authentications.Load(), commands.Load())
			}
		})
	}
}
