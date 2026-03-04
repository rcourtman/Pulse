package hostagent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestPersistAndLoadBuffer(t *testing.T) {
	dir := t.TempDir()

	agent := &Agent{
		logger:       zerolog.Nop(),
		stateDir:     dir,
		reportBuffer: utils.New[agentshost.Report](10),
	}

	r1 := agentshost.Report{
		Host:      agentshost.HostInfo{Hostname: "host1"},
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	r2 := agentshost.Report{
		Host:      agentshost.HostInfo{Hostname: "host2"},
		Timestamp: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	agent.reportBuffer.Push(r1)
	agent.reportBuffer.Push(r2)

	// Persist
	agent.persistBuffer()

	// Verify file exists
	path := filepath.Join(dir, bufferFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("buffer file should exist: %v", err)
	}

	// Create a new agent and load
	agent2 := &Agent{
		logger:       zerolog.Nop(),
		stateDir:     dir,
		reportBuffer: utils.New[agentshost.Report](10),
	}
	agent2.loadPersistedBuffer()

	if agent2.reportBuffer.Len() != 2 {
		t.Fatalf("expected 2 items in buffer, got %d", agent2.reportBuffer.Len())
	}

	// File should be deleted after loading
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("buffer file should be deleted after loading")
	}

	// Verify items are in order
	item1, _ := agent2.reportBuffer.Pop()
	if item1.Host.Hostname != "host1" {
		t.Fatalf("expected host1, got %q", item1.Host.Hostname)
	}
	item2, _ := agent2.reportBuffer.Pop()
	if item2.Host.Hostname != "host2" {
		t.Fatalf("expected host2, got %q", item2.Host.Hostname)
	}
}

func TestPersistBuffer_EmptyBuffer(t *testing.T) {
	dir := t.TempDir()

	agent := &Agent{
		logger:       zerolog.Nop(),
		stateDir:     dir,
		reportBuffer: utils.New[agentshost.Report](10),
	}

	agent.persistBuffer()

	// No file should be created for empty buffer
	path := filepath.Join(dir, bufferFileName)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("no file should be created for empty buffer")
	}
}

func TestPersistBuffer_NoStateDir(t *testing.T) {
	agent := &Agent{
		logger:       zerolog.Nop(),
		stateDir:     "",
		reportBuffer: utils.New[agentshost.Report](10),
	}

	agent.reportBuffer.Push(agentshost.Report{Host: agentshost.HostInfo{Hostname: "test"}})

	// Should not panic when no state dir
	agent.persistBuffer()
}

func TestLoadPersistedBuffer_CorruptFile(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, bufferFileName)
	if err := os.WriteFile(path, []byte("not json{{{"), 0600); err != nil {
		t.Fatal(err)
	}

	agent := &Agent{
		logger:       zerolog.Nop(),
		stateDir:     dir,
		reportBuffer: utils.New[agentshost.Report](10),
	}

	agent.loadPersistedBuffer()

	if agent.reportBuffer.Len() != 0 {
		t.Fatal("buffer should be empty after corrupt file")
	}

	// File should be deleted
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("corrupt file should be deleted")
	}
}

func TestLoadPersistedBuffer_MissingFile(t *testing.T) {
	dir := t.TempDir()

	agent := &Agent{
		logger:       zerolog.Nop(),
		stateDir:     dir,
		reportBuffer: utils.New[agentshost.Report](10),
	}

	// Should not error when file doesn't exist
	agent.loadPersistedBuffer()

	if agent.reportBuffer.Len() != 0 {
		t.Fatal("buffer should be empty when no persisted file")
	}
}

func TestPersistBuffer_AtomicWrite(t *testing.T) {
	dir := t.TempDir()

	agent := &Agent{
		logger:       zerolog.Nop(),
		stateDir:     dir,
		reportBuffer: utils.New[agentshost.Report](10),
	}

	agent.reportBuffer.Push(agentshost.Report{Host: agentshost.HostInfo{Hostname: "test"}})
	agent.persistBuffer()

	// Temp file should not exist after successful persist
	tmpPath := filepath.Join(dir, bufferFileName+".tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("temp file should not exist after successful persist")
	}

	// Verify file permissions
	path := filepath.Join(dir, bufferFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected 0600 permissions, got %o", perm)
	}
}
