package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

const (
	agentReportSizeEncodedBody = "encoded_http_body"
	agentReportSizeDecodedJSON = "decoded_json"
	agentReportSizeItems       = "decoded_items"
	agentReportSizeRecords     = "decoded_records"

	// Bytes alone do not bound what a report costs the server. An empty "{}" or
	// "null" element is a few bytes of JSON and still decodes into a full
	// struct, and each inventory record, however short, becomes a tracked
	// resource that costs tens of kilobytes across ingest, state and snapshots.
	// Reports are therefore also held to the two counts below, derived from the
	// decoded byte ceiling and generous enough that real inventories reach the
	// byte ceiling first.

	// agentReportDecodedBytesPerItem allows one item, an object or an array
	// element at any depth, for this many bytes of the decoded limit. Short
	// strings count too, such as the environment of a Docker container.
	agentReportDecodedBytesPerItem = 32
	// agentReportDecodedBytesPerRecord allows one record, an element of an
	// array held directly by the report object such as a pod, a container or
	// a disk, for this many bytes of the decoded limit. The Kubernetes agent
	// caps most of its lists at 1,000 records each, which stays well inside
	// the 40,960 that its 10 MiB limit allows.
	agentReportDecodedBytesPerRecord = 512
)

// decodeCompressedAgentReport applies independent encoded and decoded body
// limits and preserves the reason a report could not be decoded. Appliance
// inventories can be large and compress well, so the two limits are distinct.
// The decoded body is also held to the item and record counts that
// decodedLimit allows, enforced on the byte stream before anything is
// allocated for them.
func decodeCompressedAgentReport(
	w http.ResponseWriter,
	r *http.Request,
	encodedLimit int64,
	decodedLimit int64,
	destination any,
) bool {
	r.Body = http.MaxBytesReader(w, r.Body, encodedLimit)
	defer r.Body.Close()
	decompressed, err := utils.DecompressBodyIfGzipped(r, decodedLimit)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		var encodingErr *utils.UnsupportedContentEncodingError
		switch {
		case errors.As(err, &maxBytesErr):
			writeAgentReportSizeError(w, agentReportSizeEncodedBody, maxBytesErr.Limit)
		case errors.As(err, &encodingErr):
			writeErrorResponse(w, http.StatusUnsupportedMediaType, "unsupported_encoding", encodingErr.Error(), nil)
		default:
			writeErrorResponse(w, http.StatusBadRequest, "invalid_compression", "Failed to decompress request body", nil)
		}
		return false
	}
	defer decompressed.Close()
	// json.Decoder buffers and validates a whole value before it decodes any of
	// it, so a report over a count limit fails here without being decoded.
	body := &agentReportItemLimitReader{reader: decompressed, counter: newAgentReportItemCounter(decodedLimit)}

	if err := json.NewDecoder(body).Decode(destination); err != nil {
		writeAgentReportDecodeError(w, err)
		return false
	}
	// json.Decoder may finish a complete top-level object before it asks the
	// reader for the proof byte above a body limit. Drain the bounded reader so
	// compressed bombs and oversized bodies cannot bypass the cap that way.
	if _, err := io.Copy(io.Discard, body); err != nil {
		writeAgentReportDecodeError(w, err)
		return false
	}
	if err := validateAgentReportTimes(destination); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_report", err.Error(), nil)
		return false
	}

	return true
}

// agentReportItemCounter counts the items and records of a JSON report as its
// bytes pass through and fails once either exceeds its limit. An item is an
// object, or an array element that is not itself an object; a record is an
// element of an array that the report object holds directly. It keeps its
// place between calls, so a document may arrive in any chunks. It does not
// validate; the JSON decoder does that on the same bytes.
type agentReportItemCounter struct {
	maxItems   int64
	maxRecords int64
	items      int64
	records    int64
	nesting    []byte // '{' or '[' for each open object or array
	inString   bool
	escaped    bool
	// element is set where an array's next element would start: after its
	// opening bracket and after each comma directly inside it.
	element bool
	err     error
}

// newAgentReportItemCounter returns a counter with the inclusive limits of a
// report whose decoded body may be decodedLimit bytes.
func newAgentReportItemCounter(decodedLimit int64) *agentReportItemCounter {
	return &agentReportItemCounter{
		maxItems:   decodedLimit / agentReportDecodedBytesPerItem,
		maxRecords: decodedLimit / agentReportDecodedBytesPerRecord,
	}
}

// add scans the next bytes of the report. It returns an
// *agentReportItemLimitError at the first item or record over a limit and on
// every call after that, which also bounds the nesting stack.
func (c *agentReportItemCounter) add(data []byte) error {
	for _, b := range data {
		if c.err != nil {
			break
		}
		if c.inString {
			switch {
			case c.escaped:
				c.escaped = false
			case b == '\\':
				c.escaped = true
			case b == '"':
				c.inString = false
			}
			continue
		}
		switch b {
		case ' ', '\t', '\r', '\n', ':':
			continue
		case ',':
			c.element = len(c.nesting) > 0 && c.nesting[len(c.nesting)-1] == '['
			continue
		case '}', ']':
			if len(c.nesting) > 0 {
				c.nesting = c.nesting[:len(c.nesting)-1]
			}
			c.element = false
			continue
		}

		// b starts a value or an object key, or continues a number or literal.
		if c.element || b == '{' {
			c.items++
		}
		if c.element && len(c.nesting) == 2 && c.nesting[0] == '{' {
			c.records++
		}
		c.element = false
		switch b {
		case '"':
			c.inString = true
		case '{':
			c.nesting = append(c.nesting, '{')
		case '[':
			c.nesting = append(c.nesting, '[')
			c.element = true
		}

		switch {
		case c.records > c.maxRecords:
			c.err = &agentReportItemLimitError{Dimension: agentReportSizeRecords, Limit: c.maxRecords}
		case c.items > c.maxItems:
			c.err = &agentReportItemLimitError{Dimension: agentReportSizeItems, Limit: c.maxItems}
		}
	}
	return c.err
}

// agentReportItemLimitError reports a body with more items or records than
// its decoded byte limit allows.
type agentReportItemLimitError struct {
	Dimension string
	Limit     int64
}

func (e *agentReportItemLimitError) Error() string {
	return "agent report " + e.Dimension + " exceeds the limit of " + strconv.FormatInt(e.Limit, 10)
}

// agentReportItemLimitReader fails the read that carries the first item or
// record over a limit, and every read after it, without handing those bytes on.
type agentReportItemLimitReader struct {
	reader  io.Reader
	counter *agentReportItemCounter
}

func (r *agentReportItemLimitReader) Read(p []byte) (int, error) {
	if r.counter.err != nil {
		return 0, r.counter.err
	}
	n, err := r.reader.Read(p)
	if limitErr := r.counter.add(p[:n]); limitErr != nil {
		return 0, limitErr
	}
	return n, err
}

func writeAgentReportDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	var decodedSizeErr *utils.DecompressedBodyTooLargeError
	var itemLimitErr *agentReportItemLimitError
	switch {
	case errors.As(err, &itemLimitErr):
		writeErrorResponse(
			w,
			http.StatusRequestEntityTooLarge,
			"report_too_large",
			"Agent report exceeds the server item limit",
			map[string]string{
				"dimension":  itemLimitErr.Dimension,
				"limitCount": strconv.FormatInt(itemLimitErr.Limit, 10),
			},
		)
	case errors.As(err, &maxBytesErr):
		writeAgentReportSizeError(w, agentReportSizeEncodedBody, maxBytesErr.Limit)
	case errors.As(err, &decodedSizeErr):
		writeAgentReportSizeError(w, agentReportSizeDecodedJSON, decodedSizeErr.Limit)
	default:
		writeErrorResponse(w, http.StatusBadRequest, "invalid_json", "Failed to decode request body", map[string]string{"error": err.Error()})
	}
}

func writeAgentReportSizeError(w http.ResponseWriter, dimension string, limit int64) {
	writeErrorResponse(
		w,
		http.StatusRequestEntityTooLarge,
		"report_too_large",
		"Agent report exceeds the server size limit",
		map[string]string{
			"dimension":  dimension,
			"limitBytes": strconv.FormatInt(limit, 10),
		},
	)
}
