package proxmox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_AuthenticateJSON_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/access/ticket" {
			t.Errorf("expected path /api2/json/access/ticket, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"ticket":"PVE:user@pve:ticket","CSRFPreventionToken":"TOKEN"}}`)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL + "/api2/json",
		httpClient: http.DefaultClient,
		auth: auth{
			user:  "user",
			realm: "pve",
		},
		config: ClientConfig{
			Password: "password",
		},
	}

	err := client.authenticate(context.Background())
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}

	if client.auth.ticket != "PVE:user@pve:ticket" {
		t.Errorf("expected ticket PVE:user@pve:ticket, got %s", client.auth.ticket)
	}
	if client.auth.csrfToken != "TOKEN" {
		t.Errorf("expected CSRF token TOKEN, got %s", client.auth.csrfToken)
	}
}

func TestClient_AuthenticateFallbackToForm(t *testing.T) {
	jsonCalled := false
	formCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") == "application/json" {
			jsonCalled = true
			w.WriteHeader(http.StatusBadRequest) // Trigger fallback
			fmt.Fprint(w, `{"error":"use form"}`)
			return
		}
		if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
			formCalled = true
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "username=user%40pve") {
				t.Errorf("expected username in form data, got %s", string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"ticket":"form-ticket","CSRFPreventionToken":"form-token"}}`)
			return
		}
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL + "/api2/json",
		httpClient: http.DefaultClient,
		auth: auth{
			user:  "user",
			realm: "pve",
		},
		config: ClientConfig{
			Password: "password",
		},
	}

	err := client.authenticate(context.Background())
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}

	if !jsonCalled || !formCalled {
		t.Errorf("expected both JSON and Form to be called, got json=%v, form=%v", jsonCalled, formCalled)
	}
}

func TestClient_GetVMs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"vmid":100,"name":"vm1","status":"running"},{"vmid":101,"name":"vm2","status":"stopped"}]}`)
	}))
	defer server.Close()

	client, _ := NewClient(ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"})
	vms, err := client.GetVMs(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetVMs failed: %v", err)
	}
	if len(vms) != 2 {
		t.Errorf("expected 2 VMs, got %d", len(vms))
	}
}

func TestClient_GetContainers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"vmid":200,"name":"ct1","status":"running"}]}`)
	}))
	defer server.Close()

	client, _ := NewClient(ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"})
	cts, err := client.GetContainers(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetContainers failed: %v", err)
	}
	if len(cts) != 1 {
		t.Errorf("expected 1 container, got %d", len(cts))
	}
}

func TestClient_GetStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"storage":"local","type":"dir","total":1000,"used":500}]}`)
	}))
	defer server.Close()

	client, _ := NewClient(ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"})
	storage, err := client.GetStorage(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetStorage failed: %v", err)
	}
	if len(storage) != 1 {
		t.Errorf("expected 1 storage, got %d", len(storage))
	}
}

func TestClient_GetContainerConfig_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer server.Close()

	client, _ := NewClient(ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"})
	config, err := client.GetContainerConfig(context.Background(), "node1", 200)
	if err != nil {
		t.Fatalf("GetContainerConfig failed: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
}
