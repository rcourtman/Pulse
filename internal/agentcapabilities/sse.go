package agentcapabilities

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const AgentSSEAccept = "text/event-stream"

// SSERecord is one parsed Server-Sent Events record from the Pulse
// Intelligence event stream.
type SSERecord struct {
	Event string
	Data  string
}

// MCPEventNotificationWriter writes one JSON-RPC notification projected from a
// Pulse Intelligence SSE record.
type MCPEventNotificationWriter func(JSONRPCRequest) error

// ScanSSERecords parses SSE records from reader and calls handle once per
// completed record. Returning false from handle stops the scan cleanly.
func ScanSSERecords(reader io.Reader, handle func(SSERecord) bool) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1<<22)

	var event string
	var dataLines []string
	dispatch := func() bool {
		if event == "" && len(dataLines) == 0 {
			return true
		}
		record := SSERecord{
			Event: event,
			Data:  strings.Join(dataLines, "\n"),
		}
		event = ""
		dataLines = nil
		if handle == nil {
			return true
		}
		return handle(record)
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if !dispatch() {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value, hasValue := strings.Cut(line, ":")
		if !hasValue {
			value = ""
		} else {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "event":
			event = value
		case "data":
			dataLines = append(dataLines, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	dispatch()
	return nil
}

// StreamAgentSSERecords opens an authenticated Pulse Intelligence SSE stream
// and scans records through the shared parser. Callers still own connection
// lifetime, retry policy, and product-specific event handling.
func StreamAgentSSERecords(ctx context.Context, client HTTPDoer, baseURL, path, token string, handle func(SSERecord) bool) error {
	req, err := NewAgentHTTPRequest(ctx, http.MethodGet, baseURL, path, token, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", AgentSSEAccept)
	resp, err := httpClient(client).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("subscribe: status %d", resp.StatusCode)
	}
	return ScanSSERecords(resp.Body, handle)
}

// IsActionableSSERecord reports whether a parsed Pulse Intelligence SSE record
// represents a product event an external agent should handle. Empty records and
// transport keepalives stay inside the shared substrate.
func IsActionableSSERecord(record SSERecord) bool {
	return record.Event != "" && record.Data != "" && !IsTransportEventKind(record.Event)
}

// StreamAgentActionableSSERecords opens an authenticated Pulse Intelligence SSE
// stream and only forwards actionable product records to handle. Reference
// clients use this when they need raw event/data records without owning
// transport-event filtering.
func StreamAgentActionableSSERecords(ctx context.Context, client HTTPDoer, baseURL, path, token string, handle func(SSERecord) bool) error {
	return StreamAgentSSERecords(ctx, client, baseURL, path, token, func(record SSERecord) bool {
		if !IsActionableSSERecord(record) {
			return true
		}
		if handle == nil {
			return true
		}
		return handle(record)
	})
}

// StreamMCPEventNotifications opens an authenticated Pulse Intelligence SSE
// stream and projects actionable product events into MCP JSON-RPC
// notifications. Transport keepalives and empty records stay filtered at the
// shared core boundary so adapters do not each carry their own event vocabulary
// or notification projection rules.
func StreamMCPEventNotifications(ctx context.Context, client HTTPDoer, baseURL, path, token string, write MCPEventNotificationWriter) error {
	var writeErr error
	err := StreamAgentActionableSSERecords(ctx, client, baseURL, path, token, func(record SSERecord) bool {
		notification, ok := NewMCPEventNotification(record.Event, []byte(record.Data))
		if !ok {
			return true
		}
		if write == nil {
			return true
		}
		if err := write(notification); err != nil {
			writeErr = err
			return false
		}
		return true
	})
	if writeErr != nil {
		return writeErr
	}
	return err
}
