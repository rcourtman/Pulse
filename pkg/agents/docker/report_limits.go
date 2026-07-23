package dockeragent

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
)

const (
	// ReportEncodedBodyLimitBytes is the inclusive maximum size of the HTTP
	// request body after content encoding. It excludes HTTP headers and framing.
	ReportEncodedBodyLimitBytes int64 = 2 * 1024 * 1024

	// ReportDecodedBodyLimitBytes is the inclusive maximum size of the JSON
	// report after gzip decompression.
	ReportDecodedBodyLimitBytes int64 = 10 * 1024 * 1024

	reportWarningNumerator   int64 = 4
	reportWarningDenominator int64 = 5

	// ReportEncodedBodyWarningBytes is the first encoded size that is at least
	// 80% of ReportEncodedBodyLimitBytes.
	ReportEncodedBodyWarningBytes = (ReportEncodedBodyLimitBytes*reportWarningNumerator + reportWarningDenominator - 1) / reportWarningDenominator

	// ReportDecodedBodyWarningBytes is the first decoded size that is at least
	// 80% of ReportDecodedBodyLimitBytes.
	ReportDecodedBodyWarningBytes = (ReportDecodedBodyLimitBytes*reportWarningNumerator + reportWarningDenominator - 1) / reportWarningDenominator
)

// ReportSizeDimension identifies which byte boundary rejected a report.
type ReportSizeDimension string

const (
	ReportSizeEncodedBody ReportSizeDimension = "encoded_http_body"
	ReportSizeDecodedBody ReportSizeDimension = "decoded_json"
)

// ReportSizeAssessment is the shared agent/server interpretation of report
// sizes. Exact limit values are accepted; only values above a limit exceed it.
type ReportSizeAssessment struct {
	EncodedBytes int64
	DecodedBytes int64
}

// AssessReportSize evaluates encoded HTTP-body and decoded JSON byte counts
// against the canonical Docker / Podman report contract.
func AssessReportSize(encodedBytes, decodedBytes int64) ReportSizeAssessment {
	return ReportSizeAssessment{
		EncodedBytes: encodedBytes,
		DecodedBytes: decodedBytes,
	}
}

// ApproachingLimit reports whether either byte count has reached its derived
// 80% early-warning threshold.
func (a ReportSizeAssessment) ApproachingLimit() bool {
	return a.EncodedBytes >= ReportEncodedBodyWarningBytes ||
		a.DecodedBytes >= ReportDecodedBodyWarningBytes
}

// ExceedsLimit reports whether either byte count is outside the inclusive
// server contract.
func (a ReportSizeAssessment) ExceedsLimit() bool {
	return a.EncodedBytes > ReportEncodedBodyLimitBytes ||
		a.DecodedBytes > ReportDecodedBodyLimitBytes
}

// ReportSizeError describes an encoded or decoded report-size rejection.
type ReportSizeError struct {
	Dimension   ReportSizeDimension
	LimitBytes  int64
	ActualBytes int64
}

func (e *ReportSizeError) Error() string {
	if e.ActualBytes <= 0 {
		return fmt.Sprintf(
			"Docker / Podman report %s exceeds the %d byte limit",
			e.Dimension,
			e.LimitBytes,
		)
	}
	return fmt.Sprintf(
		"Docker / Podman report %s is %d bytes and exceeds the %d byte limit",
		e.Dimension,
		e.ActualBytes,
		e.LimitBytes,
	)
}

// UnsupportedReportContentEncodingError describes a content encoding that the
// Docker / Podman report endpoint does not accept.
type UnsupportedReportContentEncodingError struct {
	Encoding string
}

func (e *UnsupportedReportContentEncodingError) Error() string {
	return fmt.Sprintf("unsupported Content-Encoding: %s", e.Encoding)
}

// DecodeReportBody validates the encoded byte ceiling and returns the decoded
// JSON bytes. Empty Content-Encoding preserves uncompressed compatibility;
// gzip additionally enforces the decoded byte ceiling.
func DecodeReportBody(encoded []byte, contentEncoding string) ([]byte, error) {
	if int64(len(encoded)) > ReportEncodedBodyLimitBytes {
		return nil, &ReportSizeError{
			Dimension:   ReportSizeEncodedBody,
			LimitBytes:  ReportEncodedBodyLimitBytes,
			ActualBytes: int64(len(encoded)),
		}
	}

	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch encoding {
	case "":
		return encoded, nil
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("open gzip report: %w", err)
		}
		defer reader.Close()

		decoded, err := io.ReadAll(io.LimitReader(reader, ReportDecodedBodyLimitBytes+1))
		if err != nil {
			return nil, fmt.Errorf("decompress gzip report: %w", err)
		}
		if int64(len(decoded)) > ReportDecodedBodyLimitBytes {
			return nil, &ReportSizeError{
				Dimension:   ReportSizeDecodedBody,
				LimitBytes:  ReportDecodedBodyLimitBytes,
				ActualBytes: int64(len(decoded)),
			}
		}
		return decoded, nil
	default:
		return nil, &UnsupportedReportContentEncodingError{Encoding: encoding}
	}
}

// ReportSizeLimitDescription derives operator-facing limit text from the same
// byte constants used for enforcement.
func ReportSizeLimitDescription() string {
	return fmt.Sprintf(
		"%s encoded HTTP body or %s decoded JSON",
		formatBinaryBytes(ReportEncodedBodyLimitBytes),
		formatBinaryBytes(ReportDecodedBodyLimitBytes),
	)
}

func formatBinaryBytes(size int64) string {
	const (
		kib = int64(1024)
		mib = 1024 * kib
	)

	switch {
	case size%mib == 0:
		return fmt.Sprintf("%d MiB", size/mib)
	case size%kib == 0:
		return fmt.Sprintf("%d KiB", size/kib)
	default:
		return fmt.Sprintf("%d bytes", size)
	}
}
