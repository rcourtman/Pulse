package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetStorageContentPreservesBackupsTemplatesAndISOs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes/node1/storage/local/content" {
			fmt.Fprint(w, `{"data":[{"volid":"local:backup/ct-100.tar","content":"backup"},{"volid":"local:vztmpl/debian.tar","content":"vztmpl"},{"volid":"local:iso/ubuntu.iso","content":"iso"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	content, err := client.GetStorageContent(context.Background(), "node1", "local")
	if err != nil {
		t.Fatalf("GetStorageContent failed: %v", err)
	}
	if len(content) != 3 {
		t.Fatalf("expected all 3 content entries, got %d", len(content))
	}
	if content[0].Content != "backup" || content[1].Content != "vztmpl" || content[2].Content != "iso" {
		t.Fatalf("unexpected content list: %+v", content)
	}
}
