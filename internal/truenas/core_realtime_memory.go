package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// CORE 13's native GUI uses the older DDP event protocol, not /api/current
// JSON-RPC. Keep this short-lived, memory-only supplement separate from the
// immutable REST inventory/History transport. Never negotiate it for SCALE or
// infer that every release accepted by the broader REST gate supports it.
func supportsCORE13RealtimeMemory(version string) bool {
	version = strings.ToUpper(strings.TrimSpace(version))
	return version == "TRUENAS-13.0" || strings.HasPrefix(version, "TRUENAS-13.0-")
}

const coreRealtimeMemoryBudget = 5 * time.Second
const coreRealtimeMessageLimit = 64

// getCORE13RealtimeMemory takes one new live observation, without a cache,
// reconnect, alternate credential or REST/modern transport downgrade. Errors
// leave independently successful reporting data usable. No credentials go to a
// plaintext socket, a redirect target or an authority other than the configured
// appliance; the regular CA/fingerprint policy applies unchanged.
func (c *Client) getCORE13RealtimeMemory(ctx context.Context) (*SystemInfo, error) {
	ctx, cancel := c.rpcOperationContext(ctx)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, coreRealtimeMemoryBudget)
	defer stop()
	if err := c.rpcMu.LockContext(ctx); err != nil {
		return nil, err
	}
	defer c.rpcMu.Unlock()
	if c.closed {
		return nil, errors.New("truenas client is closed")
	}
	if c.mode != TransportLegacyREST || !supportsCORE13RealtimeMemory(c.TransportStatus().ApplianceVersion) || !c.config.UseHTTPS {
		return nil, nil
	}
	endpoint, err := url.Parse(c.rpcURL)
	if err != nil || endpoint.Scheme != "wss" {
		return nil, errors.New("truenas CORE memory requires TLS")
	}
	endpoint.Path = "/websocket"
	endpoint.RawPath, endpoint.RawQuery, endpoint.Fragment = "", "", ""
	conn, err := c.dialRPCEndpoint(ctx, endpoint.String(), true)
	if err != nil {
		return nil, err
	}
	defer conn.Close() // closing also ends the subscription; no persistent reader
	conn.SetReadLimit(maxResponseBodyBytes)
	stopContext := (&trueNASRPCClient{conn: conn}).armContext(ctx)
	defer stopContext()
	stream := coreMemoryStream{conn: conn, remaining: coreRealtimeMessageLimit}
	if err := conn.WriteJSON(map[string]any{"msg": "connect", "version": "1", "support": []string{"1"}}); err != nil {
		return nil, err
	}
	message, err := stream.read()
	if err != nil {
		return nil, err
	}
	if message.Msg != "connected" {
		return nil, errors.New("truenas CORE memory handshake rejected")
	}

	method, mechanism := "auth.login_with_api_key", "core-ddp-api-key"
	params := []string{strings.TrimSpace(c.config.APIKey)}
	if params[0] == "" {
		method, mechanism = "auth.login", "core-ddp-password"
		params = []string{strings.TrimSpace(c.config.Username), c.config.Password}
		if params[0] == "" || params[1] == "" {
			return nil, &RPCAuthError{Mechanism: mechanism, ResponseType: "missing_credentials"}
		}
	}
	const authID = "pulse-memory-auth"
	if err := conn.WriteJSON(map[string]any{"msg": "method", "id": authID, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		message, err = stream.read()
		if err != nil {
			return nil, err
		}
		if message.Msg != "result" || message.ID != authID {
			continue
		}
		var authenticated bool
		if message.hasError() || json.Unmarshal(message.Result, &authenticated) != nil || !authenticated {
			return nil, &RPCAuthError{Mechanism: mechanism, ResponseType: "rejected"}
		}
		break
	}
	const subID = "pulse-memory-realtime"
	// The unparameterized native collection uses the appliance's default
	// interval. Do not guess JSON-RPC interval syntax for the legacy protocol.
	if err := conn.WriteJSON(map[string]any{"msg": "sub", "id": subID, "name": "reporting.realtime"}); err != nil {
		return nil, err
	}
	for {
		message, err = stream.read()
		if err != nil {
			return nil, err
		}
		if message.Msg == "nosub" && message.ID == subID {
			return nil, errors.New("truenas CORE memory subscription ended before data")
		}
		if message.Msg != "added" || message.Collection != "reporting.realtime" {
			continue
		}
		// Never combine fields across frames or splice an RRD ARC into this Free.
		if memory := parseCORE13RealtimeMemory(message.Fields, time.Now().UTC()); memory != nil {
			return memory, nil
		}
		return nil, errNoSystemTelemetry
	}
}

type coreMemoryMessage struct {
	Msg        string          `json:"msg"`
	ID         string          `json:"id"`
	Collection string          `json:"collection"`
	Result     json.RawMessage `json:"result"`
	Error      json.RawMessage `json:"error"`
	Fields     json.RawMessage `json:"fields"`
}

func (m coreMemoryMessage) hasError() bool { return len(m.Error) > 0 && string(m.Error) != "null" }

type coreMemoryStream struct {
	conn      *websocket.Conn
	remaining int
}

func (s *coreMemoryStream) read() (coreMemoryMessage, error) {
	for s.remaining > 0 {
		s.remaining--
		var message coreMemoryMessage
		if err := s.conn.ReadJSON(&message); err != nil {
			return message, err
		}
		if message.Msg == "ping" {
			if err := s.conn.WriteJSON(map[string]any{"msg": "pong", "id": message.ID}); err != nil {
				return coreMemoryMessage{}, err
			}
			continue
		}
		return message, nil
	}
	return coreMemoryMessage{}, fmt.Errorf("truenas CORE memory exceeded %d messages", coreRealtimeMessageLimit)
}

// The native CORE event has separate classes.free, zfs.arc_size/classes.arc
// and virtual_memory.total. Wired includes non-ARC kernel allocations and
// virtual_memory.available includes inactive pages: neither is the Free/ARC
// reading. Integer pointers preserve missing/null versus genuine zero and
// reject numeric strings, fractions and int64 overflow at the wire boundary.
func parseCORE13RealtimeMemory(fields json.RawMessage, collectedAt time.Time) *SystemInfo {
	var frame struct {
		ZFS struct {
			ARC *int64 `json:"arc_size"`
		} `json:"zfs"`
		Memory struct {
			Classes struct {
				Free *int64 `json:"free"`
				ARC  *int64 `json:"arc"`
			} `json:"classes"`
		} `json:"memory"`
		VirtualMemory struct {
			Total *int64 `json:"total"`
		} `json:"virtual_memory"`
	}
	if json.Unmarshal(fields, &frame) != nil {
		return nil
	}
	total, free, arc := frame.VirtualMemory.Total, frame.Memory.Classes.Free, frame.ZFS.ARC
	if arc == nil {
		arc = frame.Memory.Classes.ARC
	}
	if total == nil || free == nil || arc == nil || *total <= 0 || *free < 0 || *free > *total || *arc < 0 || *arc > *total-*free {
		return nil
	}
	if frame.ZFS.ARC != nil && frame.Memory.Classes.ARC != nil && *frame.ZFS.ARC != *frame.Memory.Classes.ARC {
		return nil
	}
	return &SystemInfo{
		MemoryTotalBytes: *total, MemoryAvailableBytes: *free, ARCSizeBytes: *arc,
		CollectedAt: collectedAt, Telemetry: &SystemTelemetryAvailability{Memory: true},
	}
}
