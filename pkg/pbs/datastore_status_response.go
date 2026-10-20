package pbs

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

var errInvalidDatastoreStatus = errors.New("invalid PBS datastore status response")

// Capacity drives both displayed health and alert recovery. Do not turn a
// missing, ambiguous or wrongly typed observation into default zero counters.
// Provider extensions remain additive; no provider body enters the error.
func decodeDatastoreStatus(body []byte) (Datastore, bool, error) {
	fail := func() (Datastore, bool, error) { return Datastore{}, false, errInvalidDatastoreStatus }
	if !utf8.Valid(body) {
		return fail()
	}
	envelope, ok := backupInventoryObject(body, "data", "errors")
	if !ok {
		return fail()
	}
	if failure, exists := envelope["errors"]; exists && !bytes.Equal(bytes.TrimSpace(failure), []byte("null")) {
		return fail()
	}
	fields, ok := backupInventoryObject(envelope["data"], "total", "used", "avail", "total-space", "used-space", "avail-space", "status", "gc-status", "deduplication-factor", "deduplication_factor")
	if !ok {
		return fail()
	}
	var status struct {
		Total      int64  `json:"total"`
		Used       int64  `json:"used"`
		Avail      int64  `json:"avail"`
		TotalSpace int64  `json:"total-space"`
		UsedSpace  int64  `json:"used-space"`
		AvailSpace int64  `json:"avail-space"`
		Status     string `json:"status"`
		GCStatus   string `json:"gc-status"`
	}
	if json.Unmarshal(envelope["data"], &status) != nil {
		return fail()
	}
	datastore := Datastore{Total: status.Total, Used: status.Used, Avail: status.Avail,
		TotalSpace: status.TotalSpace, UsedSpace: status.UsedSpace, AvailSpace: status.AvailSpace,
		Status: status.Status, GCStatus: status.GCStatus}
	for _, key := range []string{"total", "used", "avail", "total-space", "used-space", "avail-space", "status", "gc-status", "deduplication-factor", "deduplication_factor"} {
		if raw, exists := fields[key]; exists && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fail()
		}
	}
	for _, counter := range []int64{datastore.Total, datastore.Used, datastore.Avail, datastore.TotalSpace, datastore.UsedSpace, datastore.AvailSpace} {
		if counter < 0 {
			return fail()
		}
	}
	// Keep the collector's existing legacy-name fallback, including inferred
	// total from positive used+available. An omitted used/available is not zero.
	_, usedPresent := fields["used"]
	_, usedLegacy := fields["used-space"]
	_, availPresent := fields["avail"]
	_, availLegacy := fields["avail-space"]
	if !(usedPresent || usedLegacy) || !(availPresent || availLegacy) {
		return fail()
	}
	total, used, avail := datastore.Total, datastore.Used, datastore.Avail
	if total == 0 {
		total = datastore.TotalSpace
	}
	if used == 0 {
		used = datastore.UsedSpace
	}
	if avail == 0 {
		avail = datastore.AvailSpace
	}
	if total == 0 && used > 0 && avail > 0 {
		if used > math.MaxInt64-avail {
			return fail()
		}
		total = used + avail
	}
	_, totalPresent := fields["total"]
	_, totalLegacy := fields["total-space"]
	// Reserved space may make used+available smaller than total, never larger.
	// Subtract only after bounding available, so neither the comparison nor
	// its intermediate values can overflow even at the int64 capacity limit.
	if (!totalPresent && !totalLegacy && total == 0) || avail > total || used > total-avail {
		return fail()
	}
	hasDedup := false
	for _, key := range []string{"deduplication_factor", "deduplication-factor"} {
		if raw, exists := fields[key]; exists {
			var factor float64
			if json.Unmarshal(raw, &factor) != nil || factor < 0 || math.IsInf(factor, 0) || math.IsNaN(factor) {
				return fail()
			}
			datastore.DeduplicationFactor = factor
			hasDedup = true
		}
	}
	datastore.Status = strings.TrimSpace(datastore.Status)
	if datastore.Status == "" {
		datastore.Status = "available"
	}
	datastore.GCStatus = strings.TrimSpace(datastore.GCStatus)
	return datastore, hasDedup, nil
}
