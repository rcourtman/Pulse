package dockeragent

import (
	"bytes"
	"compress/gzip"
	"errors"
	"testing"
)

func TestDockerReportSizeContractExactBytes(t *testing.T) {
	if ReportEncodedBodyLimitBytes != 2_097_152 {
		t.Fatalf("encoded limit = %d, want 2097152", ReportEncodedBodyLimitBytes)
	}
	if ReportDecodedBodyLimitBytes != 10_485_760 {
		t.Fatalf("decoded limit = %d, want 10485760", ReportDecodedBodyLimitBytes)
	}
	if ReportEncodedBodyWarningBytes != 1_677_722 {
		t.Fatalf("encoded warning = %d, want 1677722", ReportEncodedBodyWarningBytes)
	}
	if ReportDecodedBodyWarningBytes != 8_388_608 {
		t.Fatalf("decoded warning = %d, want 8388608", ReportDecodedBodyWarningBytes)
	}
	if got := ReportSizeLimitDescription(); got != "2 MiB encoded HTTP body or 10 MiB decoded JSON" {
		t.Fatalf("limit description = %q", got)
	}
}

func TestAssessDockerReportSizeBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		encoded     int64
		decoded     int64
		approaching bool
		exceeds     bool
	}{
		{
			name:    "ordinary report",
			encoded: 64 * 1024,
			decoded: 512 * 1024,
		},
		{
			name:    "old 890 KiB warning case is now ordinary",
			encoded: 100 * 1024,
			decoded: 890 * 1024,
		},
		{
			name:        "encoded byte below warning",
			encoded:     ReportEncodedBodyWarningBytes - 1,
			decoded:     ReportDecodedBodyWarningBytes - 1,
			approaching: false,
		},
		{
			name:        "encoded warning boundary",
			encoded:     ReportEncodedBodyWarningBytes,
			approaching: true,
		},
		{
			name:        "decoded warning boundary",
			decoded:     ReportDecodedBodyWarningBytes,
			approaching: true,
		},
		{
			name:        "exact encoded limit is accepted",
			encoded:     ReportEncodedBodyLimitBytes,
			approaching: true,
		},
		{
			name:        "exact decoded limit is accepted",
			decoded:     ReportDecodedBodyLimitBytes,
			approaching: true,
		},
		{
			name:        "encoded byte over limit",
			encoded:     ReportEncodedBodyLimitBytes + 1,
			approaching: true,
			exceeds:     true,
		},
		{
			name:        "decoded byte over limit",
			decoded:     ReportDecodedBodyLimitBytes + 1,
			approaching: true,
			exceeds:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment := AssessReportSize(test.encoded, test.decoded)
			if got := assessment.ApproachingLimit(); got != test.approaching {
				t.Fatalf("ApproachingLimit() = %v, want %v", got, test.approaching)
			}
			if got := assessment.ExceedsLimit(); got != test.exceeds {
				t.Fatalf("ExceedsLimit() = %v, want %v", got, test.exceeds)
			}
		})
	}
}

func TestDecodeDockerReportBodyBoundaries(t *testing.T) {
	t.Run("uncompressed exact encoded limit", func(t *testing.T) {
		body := bytes.Repeat([]byte("x"), int(ReportEncodedBodyLimitBytes))
		decoded, err := DecodeReportBody(body, "")
		if err != nil {
			t.Fatalf("DecodeReportBody: %v", err)
		}
		if len(decoded) != len(body) {
			t.Fatalf("decoded length = %d, want %d", len(decoded), len(body))
		}
	})

	t.Run("uncompressed byte over encoded limit", func(t *testing.T) {
		body := bytes.Repeat([]byte("x"), int(ReportEncodedBodyLimitBytes+1))
		_, err := DecodeReportBody(body, "")
		var sizeErr *ReportSizeError
		if !errors.As(err, &sizeErr) {
			t.Fatalf("error = %v, want ReportSizeError", err)
		}
		if sizeErr.Dimension != ReportSizeEncodedBody ||
			sizeErr.LimitBytes != ReportEncodedBodyLimitBytes ||
			sizeErr.ActualBytes != ReportEncodedBodyLimitBytes+1 {
			t.Fatalf("size error = %#v", sizeErr)
		}
	})

	t.Run("gzip exact decoded limit", func(t *testing.T) {
		body := bytes.Repeat([]byte("x"), int(ReportDecodedBodyLimitBytes))
		encoded := gzipDockerReportBody(t, body)
		decoded, err := DecodeReportBody(encoded, " GZip ")
		if err != nil {
			t.Fatalf("DecodeReportBody: %v", err)
		}
		if len(decoded) != len(body) {
			t.Fatalf("decoded length = %d, want %d", len(decoded), len(body))
		}
	})

	t.Run("gzip byte over decoded limit", func(t *testing.T) {
		body := bytes.Repeat([]byte("x"), int(ReportDecodedBodyLimitBytes+1))
		encoded := gzipDockerReportBody(t, body)
		_, err := DecodeReportBody(encoded, "gzip")
		var sizeErr *ReportSizeError
		if !errors.As(err, &sizeErr) {
			t.Fatalf("error = %v, want ReportSizeError", err)
		}
		if sizeErr.Dimension != ReportSizeDecodedBody ||
			sizeErr.LimitBytes != ReportDecodedBodyLimitBytes ||
			sizeErr.ActualBytes != ReportDecodedBodyLimitBytes+1 {
			t.Fatalf("size error = %#v", sizeErr)
		}
	})

	t.Run("unsupported encoding", func(t *testing.T) {
		_, err := DecodeReportBody([]byte("{}"), "zstd")
		var encodingErr *UnsupportedReportContentEncodingError
		if !errors.As(err, &encodingErr) {
			t.Fatalf("error = %v, want UnsupportedReportContentEncodingError", err)
		}
		if encodingErr.Encoding != "zstd" {
			t.Fatalf("encoding = %q, want zstd", encodingErr.Encoding)
		}
	})

	t.Run("malformed gzip", func(t *testing.T) {
		if _, err := DecodeReportBody([]byte("not-gzip"), "gzip"); err == nil {
			t.Fatal("expected malformed gzip error")
		}
	})
}

func gzipDockerReportBody(t *testing.T, body []byte) []byte {
	t.Helper()

	var encoded bytes.Buffer
	writer := gzip.NewWriter(&encoded)
	if _, err := writer.Write(body); err != nil {
		t.Fatalf("write gzip body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip body: %v", err)
	}
	return encoded.Bytes()
}
