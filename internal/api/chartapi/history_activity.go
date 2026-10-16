package chartapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
)

// ExtendedHistoryReadObserver receives only the fact that a qualifying response
// was written. No request, tenant, resource, metric, range or actor is passed.
type ExtendedHistoryReadObserver interface {
	RecordSuccessfulExtendedHistoryRead()
}

func historyDataModeCacheKey(key string) string {
	if mock.IsMockEnabled() {
		return key + "|mock"
	}
	return key
}

// WriteHistoryJSON preserves the JSON encoder's newline while sharing the same
// successful-response observation boundary as cached chart payloads.
func (r *Service) WriteHistoryJSON(w http.ResponseWriter, req *http.Request, value any, duration time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.WriteHistoryPayload(w, req, append(payload, '\n'), duration)
}

// WriteHistoryPayload counts successful GET responses, including cache hits,
// only when an actual returned metric is older than seven days. Requesting a
// long range, empty results, HEAD, cancellation, mock data and failed writes
// cannot create usage. HTTP completion does not prove a person viewed a chart.
func (r *Service) WriteHistoryPayload(w http.ResponseWriter, req *http.Request, payload []byte, duration time.Duration) error {
	observedAt := time.Now().UTC()
	n, err := w.Write(payload)
	if err != nil {
		return err
	}
	if n != len(payload) {
		return io.ErrShortWrite
	}
	if req.Method != http.MethodGet || req.Context().Err() != nil || duration <= 7*24*time.Hour || mock.IsMockEnabled() {
		return nil
	}
	observer, ok := r.resolver.(ExtendedHistoryReadObserver)
	if !ok {
		return nil
	}
	if containsExtendedHistoryPayload(payload, observedAt.Add(-7*24*time.Hour).UnixMilli()) {
		observer.RecordSuccessfulExtendedHistoryRead()
	}
	return nil
}

func containsExtendedHistoryPayload(payload []byte, cutoff int64) bool {
	if !json.Valid(payload) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if token, _ := decoder.Token(); token != json.Delim('{') {
		return false
	}
	for decoder.More() {
		key, _ := decoder.Token()
		switch key {
		case "points", "metrics", "data", "nodeData", "storageData", "dockerData", "dockerHostData", "agentData", "pools", "disks":
			if found, _ := scanExtendedHistoryPoint(decoder, cutoff); found {
				return true
			}
		default:
			var discard json.RawMessage
			if decoder.Decode(&discard) != nil {
				return false
			}
		}
	}
	return false
}

// Scan only the metric containers in the known response contracts. Streaming
// avoids retaining a second copy of a large decoded chart and stops at the
// first qualifying point. Metadata and requested window bounds cannot count.
func scanExtendedHistoryPoint(decoder *json.Decoder, cutoff int64) (bool, any) {
	token, err := decoder.Token()
	if err != nil {
		return false, nil
	}
	switch token {
	case json.Delim('{'):
		var timestamp float64
		var hasValue bool
		for decoder.More() {
			key, _ := decoder.Token()
			found, scalar := scanExtendedHistoryPoint(decoder, cutoff)
			if found {
				return true, nil
			}
			if number, ok := scalar.(float64); ok {
				if key == "timestamp" {
					timestamp = number
				} else if key == "value" {
					hasValue = true
				}
			}
		}
		_, _ = decoder.Token()
		return hasValue && timestamp > 0 && timestamp < float64(cutoff), nil
	case json.Delim('['):
		for decoder.More() {
			if found, _ := scanExtendedHistoryPoint(decoder, cutoff); found {
				return true, nil
			}
		}
		_, _ = decoder.Token()
		return false, nil
	}
	return false, token
}
