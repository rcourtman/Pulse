package availabilityprobe

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/miekg/dns"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func startTestDNSServer(t *testing.T, network string, handler dns.HandlerFunc) (string, int) {
	t.Helper()
	ready := make(chan struct{})
	srv := &dns.Server{
		Net:               network,
		Handler:           handler,
		NotifyStartedFunc: func() { close(ready) },
	}
	var addr string
	switch network {
	case "udp":
		pc, err := net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen udp: %v", err)
		}
		srv.PacketConn = pc
		addr = pc.LocalAddr().String()
	case "tcp":
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen tcp: %v", err)
		}
		srv.Listener = ln
		addr = ln.Addr().String()
	default:
		t.Fatalf("unsupported network %q", network)
	}

	go func() {
		_ = srv.ActivateAndServe()
	}()
	<-ready
	t.Cleanup(func() {
		_ = srv.Shutdown()
	})

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := net.LookupPort(network, portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return host, port
}

func TestDetailedResult_DNS_SuccessAndExpectedValues(t *testing.T) {
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		resp := new(dns.Msg)
		resp.SetReply(r)
		if len(r.Question) == 0 {
			_ = w.WriteMsg(resp)
			return
		}
		q := r.Question[0]
		switch {
		case q.Name == "nas.home.arpa." && q.Qtype == dns.TypeA:
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP("192.168.1.50"),
			})
		case q.Name == "nas.home.arpa." && q.Qtype == dns.TypeAAAA:
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
				AAAA: net.ParseIP("2001:db8::50"),
			})
		case q.Name == "txt.home.arpa." && q.Qtype == dns.TypeTXT:
			resp.Answer = append(resp.Answer, &dns.TXT{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
				Txt: []string{"v=spf1 include:example.com ~all"},
			})
		case q.Name == "mail.home.arpa." && q.Qtype == dns.TypeMX:
			resp.Answer = append(resp.Answer, &dns.MX{
				Hdr:        dns.RR_Header{Name: q.Name, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: 60},
				Preference: 10,
				Mx:         "mx1.home.arpa.",
			})
		case q.Name == "missing.home.arpa.":
			resp.SetRcode(r, dns.RcodeNameError)
		case q.Name == "broken.home.arpa.":
			resp.SetRcode(r, dns.RcodeServerFailure)
		case q.Name == "empty.home.arpa.":
			// NOERROR with zero answers
		}
		_ = w.WriteMsg(resp)
	})

	udpHost, udpPort := startTestDNSServer(t, "udp", handler)
	tcpHost, tcpPort := startTestDNSServer(t, "tcp", handler)

	t.Run("UDP A record match", func(t *testing.T) {
		res, err := DetailedResult(context.Background(), config.AvailabilityTarget{
			Address:  udpHost,
			Protocol: config.AvailabilityProbeDNS,
			Port:     udpPort,
			DNS: &config.AvailabilityDNSConfig{
				QueryName:     "nas.home.arpa",
				RecordType:    "A",
				Transport:     config.AvailabilityDNSTransportUDP,
				ExpectedValue: "192.168.1.50",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != OutcomeReachable || res.TransportOutcome != OutcomeReachable {
			t.Fatalf("unexpected outcomes: %+v", res)
		}
		if res.Application == nil || res.Application.Outcome != ApplicationPassed {
			t.Fatalf("expected application passed, got %+v", res.Application)
		}
	})

	t.Run("TCP AAAA record canonical IP match", func(t *testing.T) {
		res, err := DetailedResult(context.Background(), config.AvailabilityTarget{
			Address:  tcpHost,
			Protocol: config.AvailabilityProbeDNS,
			Port:     tcpPort,
			DNS: &config.AvailabilityDNSConfig{
				QueryName:     "nas.home.arpa",
				RecordType:    "AAAA",
				Transport:     config.AvailabilityDNSTransportTCP,
				ExpectedValue: "2001:0db8:0000:0000:0000:0000:0000:0050",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Outcome != OutcomeReachable || res.Application == nil || res.Application.Outcome != ApplicationPassed {
			t.Fatalf("unexpected result: %+v", res)
		}
	})

	t.Run("TXT substring match and MX target match", func(t *testing.T) {
		res, err := DetailedResult(context.Background(), config.AvailabilityTarget{
			Address:  udpHost,
			Protocol: config.AvailabilityProbeDNS,
			Port:     udpPort,
			DNS: &config.AvailabilityDNSConfig{
				QueryName:     "txt.home.arpa",
				RecordType:    "TXT",
				ExpectedValue: "v=spf1",
			},
		})
		if err != nil || res.Outcome != OutcomeReachable {
			t.Fatalf("expected TXT match, got res=%+v err=%v", res, err)
		}

		res, err = DetailedResult(context.Background(), config.AvailabilityTarget{
			Address:  udpHost,
			Protocol: config.AvailabilityProbeDNS,
			Port:     udpPort,
			DNS: &config.AvailabilityDNSConfig{
				QueryName:     "mail.home.arpa",
				RecordType:    "MX",
				ExpectedValue: "mx1.home.arpa",
			},
		})
		if err != nil || res.Outcome != OutcomeReachable {
			t.Fatalf("expected MX match, got res=%+v err=%v", res, err)
		}
	})

	t.Run("expected value mismatch separates transport from application failure", func(t *testing.T) {
		res, err := DetailedResult(context.Background(), config.AvailabilityTarget{
			Address:  udpHost,
			Protocol: config.AvailabilityProbeDNS,
			Port:     udpPort,
			DNS: &config.AvailabilityDNSConfig{
				QueryName:     "nas.home.arpa",
				RecordType:    "A",
				ExpectedValue: "10.0.0.1",
			},
		})
		if err == nil {
			t.Fatalf("expected mismatch error")
		}
		if res.TransportOutcome != OutcomeReachable {
			t.Fatalf("expected transport reachable, got %q", res.TransportOutcome)
		}
		if res.Outcome != OutcomeUnreachable || res.Application == nil || res.Application.FailureCode != "dns_value_mismatch" {
			t.Fatalf("unexpected application failure: %+v", res)
		}
	})

	t.Run("NXDOMAIN and SERVFAIL surface rcode application failure", func(t *testing.T) {
		res, err := DetailedResult(context.Background(), config.AvailabilityTarget{
			Address:  udpHost,
			Protocol: config.AvailabilityProbeDNS,
			Port:     udpPort,
			DNS: &config.AvailabilityDNSConfig{
				QueryName:  "missing.home.arpa",
				RecordType: "A",
			},
		})
		if err == nil || !strings.Contains(err.Error(), "NXDOMAIN") {
			t.Fatalf("expected NXDOMAIN error, got %v", err)
		}
		if res.TransportOutcome != OutcomeReachable || res.Application == nil || res.Application.FailureCode != "dns_rcode_nxdomain" {
			t.Fatalf("unexpected NXDOMAIN result: %+v", res)
		}

		res, err = DetailedResult(context.Background(), config.AvailabilityTarget{
			Address:  udpHost,
			Protocol: config.AvailabilityProbeDNS,
			Port:     udpPort,
			DNS: &config.AvailabilityDNSConfig{
				QueryName:  "broken.home.arpa",
				RecordType: "A",
			},
		})
		if err == nil || !strings.Contains(err.Error(), "SERVFAIL") {
			t.Fatalf("expected SERVFAIL error, got %v", err)
		}
		if res.TransportOutcome != OutcomeReachable || res.Application == nil || res.Application.FailureCode != "dns_rcode_servfail" {
			t.Fatalf("unexpected SERVFAIL result: %+v", res)
		}
	})

	t.Run("NOERROR with zero matching records fails as dns_no_records", func(t *testing.T) {
		res, err := DetailedResult(context.Background(), config.AvailabilityTarget{
			Address:  udpHost,
			Protocol: config.AvailabilityProbeDNS,
			Port:     udpPort,
			DNS: &config.AvailabilityDNSConfig{
				QueryName:  "empty.home.arpa",
				RecordType: "A",
			},
		})
		if err == nil {
			t.Fatalf("expected empty-records error")
		}
		if res.TransportOutcome != OutcomeReachable || res.Application == nil || res.Application.FailureCode != "dns_no_records" {
			t.Fatalf("unexpected empty-records result: %+v", res)
		}
	})
}
