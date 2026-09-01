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
)

// decodeCompressedAgentReport applies independent encoded and decoded body
// limits and preserves the reason a report could not be decoded. Appliance
// inventories can be large and compress well, so the two limits are distinct.
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

	if err := json.NewDecoder(decompressed).Decode(destination); err != nil {
		writeAgentReportDecodeError(w, err)
		return false
	}
	// json.Decoder may finish a complete top-level object before it asks the
	// reader for the proof byte above a body limit. Drain the bounded reader so
	// compressed bombs and oversized bodies cannot bypass the cap that way.
	if _, err := io.Copy(io.Discard, decompressed); err != nil {
		writeAgentReportDecodeError(w, err)
		return false
	}

	return true
}

func writeAgentReportDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	var decodedSizeErr *utils.DecompressedBodyTooLargeError
	switch {
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
