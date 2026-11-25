package monitoring

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type storageErrorStubClient struct {
	PVEClientInterface
	storageErr error
}

func (s *storageErrorStubClient) GetAllStorage(context.Context) ([]proxmox.Storage, error) {
	return nil, nil
}

func (s *storageErrorStubClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return nil, s.storageErr
}

func TestIsStorageQueryTimeoutError(t *testing.T) {
	if isStorageQueryTimeoutError(nil) {
		t.Fatal("nil error should not be a timeout")
	}
	if !isStorageQueryTimeoutError(context.DeadlineExceeded) {
		t.Fatal("context.DeadlineExceeded should be classified as timeout")
	}
	if !isStorageQueryTimeoutError(errors.New("Get https://pve:8006/api2/json/nodes/pve1/storage: net/http: timeout awaiting response headers")) {
		t.Fatal("response header timeout should be classified as timeout")
	}
	if isStorageQueryTimeoutError(errors.New("API error 403: permission check failed")) {
		t.Fatal("403 permission error should not be classified as timeout")
	}
}

func TestPollStorageWithNodes_TimeoutDoesNotBlameDatastoreAuditPermissions(t *testing.T) {
	var logBuf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&logBuf)
	defer func() { log.Logger = origLogger }()

	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{{Name: "pve1", DisableCeph: true}},
		},
		state: models.NewState(),
	}
	nodes := []proxmox.Node{{Node: "pve1", Status: "online"}}
	client := &storageErrorStubClient{
		storageErr: errors.New("Get https://pve:8006/api2/json/nodes/pve1/storage: net/http: timeout awaiting response headers"),
	}

	m.pollStorageWithNodes(context.Background(), "pve1", client, nodes)

	logs := logBuf.String()
	if !strings.Contains(logs, "All nodes timed out retrieving storage") {
		t.Fatalf("expected timeout summary log, got: %s", logs)
	}
	if strings.Contains(logs, "Datastore.Audit") {
		t.Fatalf("timeout-only storage failure must not log Datastore.Audit permission warning, got: %s", logs)
	}
}

func TestPollStorageWithNodes_PermissionFailureStillLogsDatastoreAuditHint(t *testing.T) {
	var logBuf bytes.Buffer
	origLogger := log.Logger
	log.Logger = zerolog.New(&logBuf)
	defer func() { log.Logger = origLogger }()

	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{{Name: "pve1", DisableCeph: true}},
		},
		state: models.NewState(),
	}
	nodes := []proxmox.Node{{Node: "pve1", Status: "online"}}
	client := &storageErrorStubClient{
		storageErr: errors.New("API error 403: permission check failed"),
	}

	m.pollStorageWithNodes(context.Background(), "pve1", client, nodes)

	logs := logBuf.String()
	if !strings.Contains(logs, "Datastore.Audit") {
		t.Fatalf("expected Datastore.Audit permission warning for 403 failure, got: %s", logs)
	}
}
