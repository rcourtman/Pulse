package availabilityprobe

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func probeDNS(ctx context.Context, target config.AvailabilityTarget, timeout time.Duration) (ProbeResult, error) {
	if target.DNS == nil {
		return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeUnreachable}, fmt.Errorf("DNS availability targets require DNS query settings")
	}
	qtype, recordType, err := resolveDNSRecordType(target.DNS.RecordType)
	if err != nil {
		return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeUnreachable}, err
	}

	port := target.Port
	if port <= 0 {
		port = config.DefaultAvailabilityDNSPort
	}
	transport := string(target.DNS.Transport)
	if transport == "" {
		transport = string(config.AvailabilityDNSTransportUDP)
	}

	serverAddr := net.JoinHostPort(target.ProbeAddress(), strconv.Itoa(port))
	queryName := strings.TrimSpace(target.DNS.QueryName)

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(queryName), qtype)
	msg.RecursionDesired = true

	client := &dns.Client{
		Net:     transport,
		Timeout: timeout,
	}

	resp, _, err := client.ExchangeContext(ctx, msg, serverAddr)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeUnreachable}, fmt.Errorf("dns probe timed out: %w", ctxErr)
		}
		return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeUnreachable}, fmt.Errorf("dns probe failed: %w", err)
	}
	if resp == nil {
		return ProbeResult{Outcome: OutcomeUnreachable, TransportOutcome: OutcomeUnreachable}, fmt.Errorf("dns server returned empty response")
	}

	result := ProbeResult{
		Outcome:          OutcomeReachable,
		TransportOutcome: OutcomeReachable,
		Application: &ApplicationResult{
			Outcome:    ApplicationPassed,
			StatusCode: resp.Rcode,
		},
	}

	if resp.Rcode != dns.RcodeSuccess {
		rcodeName := dnsRcodeLabel(resp.Rcode)
		return applicationFailure(
			result,
			"dns_rcode_"+strings.ToLower(rcodeName),
			fmt.Sprintf("dns server returned %s for %s %s", rcodeName, recordType, queryName),
		)
	}

	answers := extractDNSAnswerValues(resp.Answer, qtype)
	if len(answers) == 0 {
		return applicationFailure(
			result,
			"dns_no_records",
			fmt.Sprintf("dns response returned no %s records for %s", recordType, queryName),
		)
	}

	expected := strings.TrimSpace(target.DNS.ExpectedValue)
	if expected != "" && !dnsAnswersMatchExpected(recordType, answers, expected) {
		return applicationFailure(
			result,
			"dns_value_mismatch",
			fmt.Sprintf("dns %s response for %s did not match expected %q (got %s)", recordType, queryName, expected, strings.Join(answers, ", ")),
		)
	}

	return result, nil
}

func resolveDNSRecordType(raw string) (uint16, string, error) {
	recordType := strings.ToUpper(strings.TrimSpace(raw))
	if recordType == "" {
		recordType = "A"
	}
	switch recordType {
	case "A":
		return dns.TypeA, "A", nil
	case "AAAA":
		return dns.TypeAAAA, "AAAA", nil
	case "CNAME":
		return dns.TypeCNAME, "CNAME", nil
	case "TXT":
		return dns.TypeTXT, "TXT", nil
	case "MX":
		return dns.TypeMX, "MX", nil
	case "NS":
		return dns.TypeNS, "NS", nil
	case "SRV":
		return dns.TypeSRV, "SRV", nil
	case "PTR":
		return dns.TypePTR, "PTR", nil
	default:
		return 0, "", fmt.Errorf("unsupported DNS record type %q", raw)
	}
}

func dnsRcodeLabel(rcode int) string {
	if name, ok := dns.RcodeToString[rcode]; ok && name != "" {
		return name
	}
	return fmt.Sprintf("RCODE_%d", rcode)
}

func extractDNSAnswerValues(answers []dns.RR, qtype uint16) []string {
	values := make([]string, 0, len(answers))
	for _, rr := range answers {
		if rr == nil || rr.Header().Rrtype != qtype {
			continue
		}
		switch record := rr.(type) {
		case *dns.A:
			if record.A != nil {
				values = append(values, record.A.String())
			}
		case *dns.AAAA:
			if record.AAAA != nil {
				values = append(values, record.AAAA.String())
			}
		case *dns.CNAME:
			values = append(values, strings.TrimSuffix(record.Target, "."))
		case *dns.TXT:
			values = append(values, strings.Join(record.Txt, ""))
		case *dns.MX:
			values = append(values, fmt.Sprintf("%d %s", record.Preference, strings.TrimSuffix(record.Mx, ".")))
		case *dns.NS:
			values = append(values, strings.TrimSuffix(record.Ns, "."))
		case *dns.SRV:
			values = append(values, fmt.Sprintf("%d %d %d %s", record.Priority, record.Weight, record.Port, strings.TrimSuffix(record.Target, ".")))
		case *dns.PTR:
			values = append(values, strings.TrimSuffix(record.Ptr, "."))
		}
	}
	return values
}

func dnsAnswersMatchExpected(recordType string, answers []string, expected string) bool {
	normalizedExpected := strings.TrimSuffix(strings.TrimSpace(expected), ".")
	if normalizedExpected == "" {
		return true
	}
	switch recordType {
	case "A", "AAAA":
		expectedIP := net.ParseIP(normalizedExpected)
		for _, answer := range answers {
			if expectedIP != nil {
				if answerIP := net.ParseIP(answer); answerIP != nil && answerIP.Equal(expectedIP) {
					return true
				}
			}
			if strings.EqualFold(answer, normalizedExpected) {
				return true
			}
		}
		return false
	case "TXT":
		for _, answer := range answers {
			if strings.Contains(answer, expected) {
				return true
			}
		}
		return false
	case "MX":
		for _, answer := range answers {
			if strings.EqualFold(answer, normalizedExpected) {
				return true
			}
			parts := strings.Fields(answer)
			if len(parts) == 2 && strings.EqualFold(parts[1], normalizedExpected) {
				return true
			}
		}
		return false
	case "SRV":
		for _, answer := range answers {
			if strings.EqualFold(answer, normalizedExpected) {
				return true
			}
			parts := strings.Fields(answer)
			if len(parts) == 4 && strings.EqualFold(parts[3], normalizedExpected) {
				return true
			}
		}
		return false
	default:
		for _, answer := range answers {
			if strings.EqualFold(strings.TrimSuffix(answer, "."), normalizedExpected) {
				return true
			}
		}
		return false
	}
}
