package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPDMHTTPClientResourceListUsesCurrentAPIAuthAndGroupedResponse(t *testing.T) {
	const (
		tokenID     = "root@pam!pulse"
		tokenSecret = "secret-value"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/extjs/resources/list" {
			t.Fatalf("path: want /api2/extjs/resources/list, got %q", r.URL.Path)
		}
		wantAuth := "PDMAPIToken " + tokenID + ":" + tokenSecret
		if got := r.Header.Get("Authorization"); got != wantAuth {
			t.Fatalf("Authorization: want %q, got %q", wantAuth, got)
		}
		if strings.Contains(r.Header.Get("Authorization"), "PVEAPIToken") {
			t.Fatal("Authorization used legacy PVE token scheme")
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"data": [
				{
					"remote": "pve-a",
					"resources": [
						{"type": "pve-qemu", "id": "remote/pve-a/guest/101", "name": "vm-101", "status": "running"},
						{"type": "pve-lxc", "id": "remote/pve-a/guest/102", "name": "ct-102", "status": "stopped"},
						{"type": "pve-node", "id": "remote/pve-a/node/pve1", "node": "pve1", "status": "online"},
						{"type": "pve-storage", "id": "remote/pve-a/storage/local", "storage": "local", "status": "offline"},
						{"type": "pve-network", "id": "remote/pve-a/network/zone/vnet1", "name": "vnet1", "status": "available"}
					]
				},
				{
					"remote": "pbs-a",
					"error": "cached remote warning",
					"resources": [
						{"type": "pbs-node", "id": "remote/pbs-a/node/pbs1", "name": "pbs1", "uptime": 12},
						{"type": "pbs-node", "id": "remote/pbs-a/node/pbs2", "name": "pbs2", "uptime": 0},
						{"type": "pbs-datastore", "id": "remote/pbs-a/datastore/fast", "name": "fast", "maintenance": null},
						{"type": "pbs-datastore", "id": "remote/pbs-a/datastore/archive", "name": "archive", "maintenance": "offline"}
					]
				}
			]
		}`)
	}))
	defer server.Close()

	client, err := newPDMHTTPClient(pdmAlertBridgeConfig{
		APIURL:         server.URL,
		APIToken:       tokenID,
		APITokenSecret: tokenSecret,
	})
	if err != nil {
		t.Fatalf("newPDMHTTPClient: %v", err)
	}

	got, err := client.ResourceList(context.Background())
	if err != nil {
		t.Fatalf("ResourceList: %v", err)
	}

	want := []pdmResource{
		{ID: "remote/pve-a/guest/101", RemoteID: "pve-a", Name: "vm-101", Type: "qemu", Status: "running"},
		{ID: "remote/pve-a/guest/102", RemoteID: "pve-a", Name: "ct-102", Type: "lxc", Status: "stopped"},
		{ID: "remote/pve-a/node/pve1", RemoteID: "pve-a", Name: "pve1", Type: "node", Status: "online"},
		{ID: "remote/pve-a/storage/local", RemoteID: "pve-a", Name: "local", Type: "storage", Status: "offline"},
		{ID: "remote/pbs-a/node/pbs1", RemoteID: "pbs-a", Name: "pbs1", Type: "node", Status: "online"},
		{ID: "remote/pbs-a/node/pbs2", RemoteID: "pbs-a", Name: "pbs2", Type: "node", Status: "offline"},
		{ID: "remote/pbs-a/datastore/fast", RemoteID: "pbs-a", Name: "fast", Type: "storage", Status: "online"},
		{ID: "remote/pbs-a/datastore/archive", RemoteID: "pbs-a", Name: "archive", Type: "storage", Status: "unknown"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resources:\nwant %#v\n got %#v", want, got)
	}
}

func TestPDMHTTPClientNonSuccessErrorDoesNotExposeToken(t *testing.T) {
	const tokenSecret = "do-not-leak"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := newPDMHTTPClient(pdmAlertBridgeConfig{
		APIURL:         server.URL,
		APIToken:       "root@pam!pulse",
		APITokenSecret: tokenSecret,
	})
	if err != nil {
		t.Fatalf("newPDMHTTPClient: %v", err)
	}

	_, err = client.ResourceList(context.Background())
	if err == nil {
		t.Fatal("ResourceList should fail on non-2xx status")
	}
	errText := err.Error()
	if strings.Contains(errText, tokenSecret) || strings.Contains(errText, "Authorization") || strings.Contains(errText, "PDMAPIToken") {
		t.Fatalf("error exposes auth material: %q", errText)
	}
}

func TestPDMAlertBridgeConfigEnvActivation(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	t.Setenv(pdmAPIURLEnv, "")
	t.Setenv(pdmAPITokenEnv, "root@pam!pulse")
	t.Setenv(pdmAPITokenSecretEnv, "secret")
	if got := newPDMAlertSourceFromEnv(); got != nil {
		t.Fatalf("incomplete env should not activate PDM source, got %T", got)
	}

	t.Setenv(pdmAPIURLEnv, server.URL)
	t.Setenv(pdmAPITokenEnv, "root@pam!pulse")
	t.Setenv(pdmAPITokenSecretEnv, "secret")
	t.Setenv(pdmInsecureSkipVerifyEnv, "true")
	source := newPDMAlertSourceFromEnv()
	client, ok := source.(*pdmHTTPClient)
	if !ok {
		t.Fatalf("complete env should activate *pdmHTTPClient, got %T", source)
	}
	if client.httpClient.Timeout != pdmHTTPClientRequestTimeout {
		t.Fatalf("timeout: want %s, got %s", pdmHTTPClientRequestTimeout, client.httpClient.Timeout)
	}

	patrol := NewPatrolService(nil, nil)
	if patrol.pdmAlertBridge == nil || patrol.pdmAlertBridge.source == nil {
		t.Fatal("NewPatrolService should wire PDM source when complete env is present")
	}
}

func TestPDMHTTPClientTLSVerificationDefaultsOn(t *testing.T) {
	client, err := newPDMHTTPClient(pdmAlertBridgeConfig{
		APIURL:         "https://pdm.example.test",
		APIToken:       "root@pam!pulse",
		APITokenSecret: "secret",
	})
	if err != nil {
		t.Fatalf("newPDMHTTPClient: %v", err)
	}
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport: want *http.Transport, got %T", client.httpClient.Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig should be set")
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS verification should be enabled by default")
	}
	if client.httpClient.Timeout != 10*time.Second {
		t.Fatalf("timeout: want 10s, got %s", client.httpClient.Timeout)
	}

	insecureClient, err := newPDMHTTPClient(pdmAlertBridgeConfig{
		APIURL:             "https://pdm.example.test",
		APIToken:           "root@pam!pulse",
		APITokenSecret:     "secret",
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("newPDMHTTPClient insecure: %v", err)
	}
	insecureTransport, ok := insecureClient.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport: want *http.Transport, got %T", insecureClient.httpClient.Transport)
	}
	if insecureTransport.TLSClientConfig == nil || !insecureTransport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("PDM_INSECURE_SKIP_VERIFY=true should opt out of default TLS verification")
	}
}
