package securityutil

import (
	"errors"
	"strings"
	"testing"
)

func TestParseProxmoxSession(t *testing.T) {
	for _, ticket := range []string{"PVE:fixture@pam:1234::base64+/=", "PBS:fixture@pbs:1234::base64+/="} {
		body := []byte(`{"data":{"ticket":"` + ticket + `","CSRFPreventionToken":"1234:base64+/=","username":"fixture@pam","cap":{"VM.Audit":1}},"errors":null}`)
		gotTicket, csrf, err := ParseProxmoxSession(body)
		if err != nil || gotTicket != ticket || csrf != "1234:base64+/=" {
			t.Fatal("complete native-format fixture with metadata was not preserved")
		}
	}
	for _, body := range []string{
		`{"data":{"ticket":"fixture-sensitive","CSRFPreventionToken":"csrf"},"errors":{"password":"rejected"}}`,
		`{"data":{"ticket":"fixture-sensitive","CSRFPreventionToken":"csrf"},"Errors":null}`,
		`{"data":{"ticket":"fixture-sensitive","CSRFPreventionToken":"csrf","username":null,"username":"fixture"}}`,
		`{"data":{"ticket":"fixture-sensitive","CSRFPreventionToken":"csrf"},"data":null}`,
		`{"data":{"ticket":"fixture-sensitive","CSRFPreventionToken":"csrf"}} []`,
		`[]`, `null`, `{"data":[]}`,
	} {
		ticket, csrf, err := ParseProxmoxSession([]byte(body))
		if !errors.Is(err, ErrInvalidProxmoxSession) || ticket != "" || csrf != "" || strings.Contains(err.Error(), "fixture-sensitive") {
			t.Fatal("invalid response admitted credentials or exposed their bytes")
		}
	}
}
