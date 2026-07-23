package api

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// zipEntry0723Am is a single in-memory zip member used by buildZipBytes0723Am.
type zipEntry0723Am struct {
	name    string
	content []byte
}

// buildZipBytes0723Am builds an in-memory zip archive containing entries in the
// given order and returns its raw bytes. Test-only fixture builder for
// extractFromZip coverage.
func buildZipBytes0723Am(t *testing.T, entries ...zipEntry0723Am) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", e.name, err)
		}
		if _, err := w.Write(e.content); err != nil {
			t.Fatalf("write zip entry %q: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// zipLocalHeaderDataOffset0723Am parses the first local file header of archive
// and returns the byte offset where that entry's compressed data begins.
func zipLocalHeaderDataOffset0723Am(t *testing.T, archive []byte) int {
	t.Helper()
	const localHeaderSig = 0x04034b50
	if len(archive) < 30 {
		t.Fatal("archive too small to contain a local file header")
	}
	if binary.LittleEndian.Uint32(archive[0:4]) != localHeaderSig {
		t.Fatal("archive does not start with a local file header signature")
	}
	nameLen := int(binary.LittleEndian.Uint16(archive[26:28]))
	extraLen := int(binary.LittleEndian.Uint16(archive[28:30]))
	return 30 + nameLen + extraLen
}

// zipCentralDirOffset0723Am returns the byte offset of the first central
// directory entry, read from the trailing EOCD record (archive/zip writes no
// comment, so the EOCD is the last 22 bytes).
func zipCentralDirOffset0723Am(t *testing.T, archive []byte) int {
	t.Helper()
	const eocdSig = 0x06054b50
	if len(archive) < 22 {
		t.Fatal("archive too small to contain an EOCD record")
	}
	eocd := len(archive) - 22
	if binary.LittleEndian.Uint32(archive[eocd:eocd+4]) != eocdSig {
		t.Fatal("EOCD signature not found at end of archive")
	}
	cdOff := int(binary.LittleEndian.Uint32(archive[eocd+16 : eocd+20]))
	if cdOff+12 > len(archive) || binary.LittleEndian.Uint32(archive[cdOff:cdOff+4]) != 0x02014b50 {
		t.Fatalf("central directory signature not found at offset %d", cdOff)
	}
	return cdOff
}

// patchZipMethodUnsupported0723Am returns a copy of archive with both the local
// and central-directory compression methods of the first entry set to 99 (an
// unsupported method), so that file.Open() fails while zip.NewReader still
// parses the structure.
func patchZipMethodUnsupported0723Am(t *testing.T, archive []byte) []byte {
	t.Helper()
	const unsupported = uint16(99)
	out := append([]byte(nil), archive...)
	binary.LittleEndian.PutUint16(out[8:10], unsupported)
	cdOff := zipCentralDirOffset0723Am(t, out)
	binary.LittleEndian.PutUint16(out[cdOff+10:cdOff+12], unsupported)
	return out
}

func TestBranchcov0723AmExtractFromZip(t *testing.T) {
	want := []byte("hello-agent-binary")

	t.Run("valid_zip_returns_exact_entry_bytes", func(t *testing.T) {
		archive := buildZipBytes0723Am(t, zipEntry0723Am{name: "pulse-agent.exe", content: want})
		got, err := extractFromZip(archive, "pulse-agent.exe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("extractFromZip returned %q, want %q", got, want)
		}
	})

	t.Run("missing_entry_returns_not_found_error", func(t *testing.T) {
		archive := buildZipBytes0723Am(t, zipEntry0723Am{name: "other.bin", content: []byte("not-the-one")})
		got, err := extractFromZip(archive, "pulse-agent.exe")
		if err == nil {
			t.Fatalf("expected error for missing entry, got content %q", got)
		}
		if !strings.Contains(err.Error(), "not found in zip") {
			t.Fatalf("expected 'not found in zip' error, got %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil content on error, got %q", got)
		}
	})

	t.Run("entry_found_when_not_first", func(t *testing.T) {
		archive := buildZipBytes0723Am(t,
			zipEntry0723Am{name: "README.txt", content: []byte("readme")},
			zipEntry0723Am{name: "LICENSE", content: []byte("license")},
			zipEntry0723Am{name: "pulse-agent.exe", content: want},
			zipEntry0723Am{name: "tail.bin", content: []byte("tail")},
		)
		got, err := extractFromZip(archive, "pulse-agent.exe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("extractFromZip returned %q, want %q", got, want)
		}
	})

	t.Run("empty_archive_returns_open_error", func(t *testing.T) {
		got, err := extractFromZip([]byte{}, "pulse-agent.exe")
		if err == nil {
			t.Fatalf("expected error for empty archive, got content %q", got)
		}
		if !strings.Contains(err.Error(), "failed to open zip") {
			t.Fatalf("expected 'failed to open zip' error, got %v", err)
		}
	})

	t.Run("non_zip_bytes_returns_open_error", func(t *testing.T) {
		archive := []byte("this is definitely not a zip file")
		got, err := extractFromZip(archive, "pulse-agent.exe")
		if err == nil {
			t.Fatalf("expected error for non-zip bytes, got content %q", got)
		}
		if !strings.Contains(err.Error(), "failed to open zip") {
			t.Fatalf("expected 'failed to open zip' error, got %v", err)
		}
	})

	t.Run("name_match_is_case_sensitive", func(t *testing.T) {
		// extractFromZip compares filepath.Base(file.Name) to entryName using a
		// verbatim byte comparison; differing case does not match.
		archive := buildZipBytes0723Am(t, zipEntry0723Am{name: "Pulse-Agent.EXE", content: want})
		if _, err := extractFromZip(archive, "pulse-agent.exe"); err == nil {
			t.Fatal("expected not-found for case-mismatched entry name, got no error")
		}
		got, err := extractFromZip(archive, "Pulse-Agent.EXE")
		if err != nil {
			t.Fatalf("unexpected error for exact-case name: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("extractFromZip returned %q, want %q", got, want)
		}
	})

	t.Run("name_matched_on_filepath_base", func(t *testing.T) {
		// The match is on filepath.Base(file.Name), so a leading directory
		// segment is stripped and a nested entry still resolves.
		archive := buildZipBytes0723Am(t, zipEntry0723Am{name: "dist/pulse-agent.exe", content: want})
		got, err := extractFromZip(archive, "pulse-agent.exe")
		if err != nil {
			t.Fatalf("expected base-name match for nested entry, got error: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("extractFromZip returned %q, want %q", got, want)
		}
	})

	t.Run("zero_length_entry_returns_empty_bytes", func(t *testing.T) {
		archive := buildZipBytes0723Am(t, zipEntry0723Am{name: "pulse-agent.exe", content: nil})
		got, err := extractFromZip(archive, "pulse-agent.exe")
		if err != nil {
			t.Fatalf("unexpected error for zero-length entry: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty byte slice, got %d bytes", len(got))
		}
	})

	t.Run("oversized_entry_returns_size_limit_error", func(t *testing.T) {
		// Deflate compresses all-zero content to a few KB, so the archive stays
		// small while the decompressed entry exceeds maxAgentBinarySize.
		big := bytes.Repeat([]byte{0}, maxAgentBinarySize+1)
		archive := buildZipBytes0723Am(t, zipEntry0723Am{name: "pulse-agent.exe", content: big})
		got, err := extractFromZip(archive, "pulse-agent.exe")
		if err == nil {
			t.Fatalf("expected size-limit error, got %d bytes", len(got))
		}
		if !strings.Contains(err.Error(), "exceeded size limit") {
			t.Fatalf("expected 'exceeded size limit' error, got %v", err)
		}
	})

	t.Run("unsupported_compression_method_returns_open_error", func(t *testing.T) {
		// Rewrite the method to an unsupported value in both headers; the
		// structure still parses but file.Open() rejects the method.
		archive := patchZipMethodUnsupported0723Am(t,
			buildZipBytes0723Am(t, zipEntry0723Am{name: "pulse-agent.exe", content: want}))
		got, err := extractFromZip(archive, "pulse-agent.exe")
		if err == nil {
			t.Fatalf("expected open error for unsupported method, got %d bytes", len(got))
		}
		if !strings.Contains(err.Error(), "failed opening binary in zip") {
			t.Fatalf("expected 'failed opening binary in zip' error, got %v", err)
		}
	})

	t.Run("corrupt_compressed_data_returns_read_error", func(t *testing.T) {
		// Flip bits inside the compressed-data region; file.Open() still
		// succeeds but decompression fails mid-read.
		base := buildZipBytes0723Am(t,
			zipEntry0723Am{name: "pulse-agent.exe", content: bytes.Repeat([]byte("payload-"), 32)})
		out := append([]byte(nil), base...)
		dataOff := zipLocalHeaderDataOffset0723Am(t, out)
		cdOff := zipCentralDirOffset0723Am(t, out)
		n := 8
		if max := cdOff - dataOff - 1; max < n {
			n = max
		}
		if n <= 0 {
			t.Fatal("no compressed-data bytes available to corrupt")
		}
		for i := 0; i < n; i++ {
			out[dataOff+i] ^= 0xFF
		}
		got, err := extractFromZip(out, "pulse-agent.exe")
		if err == nil {
			t.Fatalf("expected read error for corrupt data, got %d bytes", len(got))
		}
		if !strings.Contains(err.Error(), "failed reading binary from zip") {
			t.Fatalf("expected 'failed reading binary from zip' error, got %v", err)
		}
	})
}

func TestBranchcov0723AmIsTrustedProxyIP(t *testing.T) {
	cases := []struct {
		name    string
		envCIDR string
		ipStr   string
		want    bool
	}{
		{
			name:    "empty string returns false",
			envCIDR: "10.0.0.0/8",
			ipStr:   "",
			want:    false,
		},
		{
			name:    "syntactically invalid IP returns false",
			envCIDR: "10.0.0.0/8",
			ipStr:   "not-an-ip",
			want:    false,
		},
		{
			name:    "IPv4 inside trusted range returns true",
			envCIDR: "10.0.0.0/8",
			ipStr:   "10.1.2.3",
			want:    true,
		},
		{
			name:    "IPv4 outside trusted range returns false",
			envCIDR: "10.0.0.0/8",
			ipStr:   "192.168.1.1",
			want:    false,
		},
		{
			name:    "IPv6 inside trusted range returns true",
			envCIDR: "2001:db8::/32",
			ipStr:   "2001:db8::1",
			want:    true,
		},
		{
			name:    "IPv6 outside trusted range returns false",
			envCIDR: "2001:db8::/32",
			ipStr:   "2001:dead::1",
			want:    false,
		},
		{
			name:    "surrounding whitespace is trimmed then matched",
			envCIDR: "10.0.0.0/8",
			ipStr:   "  10.1.2.3  ",
			want:    true,
		},
		{
			name:    "bracketed IPv6 is unbracketed then matched",
			envCIDR: "2001:db8::/32",
			ipStr:   "[2001:db8::1]",
			want:    true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", tt.envCIDR)
			resetTrustedProxyConfig()

			if got := IsTrustedProxyIP(tt.ipStr); got != tt.want {
				t.Errorf("IsTrustedProxyIP(%q) = %v, want %v", tt.ipStr, got, tt.want)
			}
		})
	}
}
