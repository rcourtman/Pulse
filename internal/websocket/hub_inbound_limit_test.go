package websocket

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHandleWebSocket_DecompressedInboundLimit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		compressed bool
		bytes      int
		fragmented bool
		unfinished bool
		wantTooBig bool
	}{
		{name: "plain at limit", bytes: maxWebSocketInboundMessageSize},
		{name: "compressed at limit", compressed: true, bytes: maxWebSocketInboundMessageSize},
		{name: "plain above limit", bytes: maxWebSocketInboundMessageSize + 1, wantTooBig: true},
		{name: "compressed above limit", compressed: true, bytes: maxWebSocketInboundMessageSize + 1, wantTooBig: true},
		{name: "compressed expansion", compressed: true, bytes: 4 * maxWebSocketInboundMessageSize, wantTooBig: true},
		{name: "compressed fragmented expansion", compressed: true, bytes: 4 * maxWebSocketInboundMessageSize, fragmented: true, wantTooBig: true},
		{name: "compressed unfinished expansion", compressed: true, bytes: 4 * maxWebSocketInboundMessageSize, unfinished: true, wantTooBig: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub := NewHub(nil)
			go hub.Run()
			t.Cleanup(hub.Stop)
			server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
			t.Cleanup(server.Close)
			dialer := websocket.Dialer{EnableCompression: tc.compressed}
			conn, response, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"?org_id=default", wsHeadersForHTTP(t, server.URL))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { conn.Close() })
			if tc.compressed && !strings.Contains(response.Header.Get("Sec-WebSocket-Extensions"), "permessage-deflate") {
				t.Fatal("compression was not negotiated")
			}
			const prefix = `{"type":"ping","data":"`
			const suffix = `"}`
			payload := []byte(prefix + strings.Repeat("x", tc.bytes-len(prefix)-len(suffix)) + suffix)
			if tc.fragmented || tc.unfinished {
				compressed := deflateInboundFixture(t, payload)
				var frames []byte
				if tc.fragmented {
					middle := len(compressed) / 2
					frames = appendInboundFixtureFrame(frames, 1, false, true, compressed[:middle])
					frames = appendInboundFixtureFrame(frames, 0, true, false, compressed[middle:])
				} else {
					frames = appendInboundFixtureFrame(frames, 1, false, true, compressed)
				}
				_, err = io.Copy(conn.UnderlyingConn(), bytes.NewReader(frames))
			} else {
				err = conn.WriteMessage(websocket.TextMessage, payload)
			}
			if err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			_, reply, err := conn.ReadMessage()
			if tc.wantTooBig {
				if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
					t.Fatalf("oversized message was not closed with 1009: reply=%s err=%v", reply, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("at-limit message was rejected: %v", err)
			}
			var message Message
			if err := json.Unmarshal(reply, &message); err != nil || message.Type != "pong" {
				t.Fatalf("at-limit ping did not receive pong: reply=%s err=%v", reply, err)
			}
		})
	}
}

// A sync-flushed raw DEFLATE stream without its final four bytes is the
// permessage-deflate payload. The largest fixture expands to only 256 KiB.
func deflateInboundFixture(t *testing.T, payload []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer, err := flate.NewWriter(&compressed, flate.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	result := bytes.Clone(compressed.Bytes())
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if len(result) < 4 || !bytes.Equal(result[len(result)-4:], []byte{0, 0, 255, 255}) {
		t.Fatal("fixture lacks the DEFLATE sync-flush trailer")
	}
	result = result[:len(result)-4]
	if len(result) >= maxWebSocketInboundMessageSize {
		t.Fatal("fixture must fit below the compressed wire limit")
	}
	return result
}

// Build masked client frames explicitly so the test can omit FIN. Rejection
// must occur before a sender finishes a message, rather than after ReadAll.
func appendInboundFixtureFrame(dst []byte, opcode byte, final, compressed bool, payload []byte) []byte {
	first := opcode
	if final {
		first |= 0x80
	}
	if compressed {
		first |= 0x40
	}
	dst = append(dst, first)
	switch {
	case len(payload) < 126:
		dst = append(dst, 0x80|byte(len(payload)))
	case len(payload) <= 65535:
		dst = append(dst, 0x80|126)
		dst = binary.BigEndian.AppendUint16(dst, uint16(len(payload)))
	default:
		dst = append(dst, 0x80|127)
		dst = binary.BigEndian.AppendUint64(dst, uint64(len(payload)))
	}
	mask := [4]byte{17, 34, 51, 68}
	dst = append(dst, mask[:]...)
	for i, value := range payload {
		dst = append(dst, value^mask[i%len(mask)])
	}
	return dst
}
