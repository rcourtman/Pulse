package providers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// Provider responses contain model output and, for local/compatible servers,
// originate outside Pulse's trust boundary. This limit is deliberately well
// above normal chat, model-list, and streaming responses while preventing a
// malformed endpoint from driving unbounded memory or bandwidth consumption.
const maxProviderResponseBodyBytes int64 = 16 << 20 // 16 MiB

func limitProviderResponseBody(resp *http.Response) error {
	if err := securityutil.LimitResponseBody(resp, maxProviderResponseBodyBytes); err != nil {
		return fmt.Errorf("limit provider response: %w", err)
	}
	return nil
}

func readProviderResponseBody(resp *http.Response) ([]byte, error) {
	if err := limitProviderResponseBody(resp); err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}
