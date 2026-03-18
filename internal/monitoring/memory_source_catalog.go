package monitoring

import "strings"

type MemorySourceDescriptor struct {
	Canonical             string
	Trust                 string
	Fallback              bool
	DefaultFallbackReason string
}

var memorySourceCatalog = map[string]MemorySourceDescriptor{
	"":                            {Canonical: "unknown", Trust: "fallback", Fallback: true},
	"unknown":                     {Canonical: "unknown", Trust: "fallback", Fallback: true},
	"nodes-endpoint":              {Canonical: "nodes-endpoint", Trust: "fallback", Fallback: true},
	"node-status-used":            {Canonical: "node-status-used", Trust: "fallback", Fallback: true},
	"previous-snapshot":           {Canonical: "previous-snapshot", Trust: "fallback", Fallback: true, DefaultFallbackReason: "preserved-previous-snapshot"},
	"available-field":             {Canonical: "available-field", Trust: "preferred"},
	"avail-field":                 {Canonical: "available-field", Trust: "preferred"},
	"meminfo-available":           {Canonical: "available-field", Trust: "preferred"},
	"node-status-available":       {Canonical: "available-field", Trust: "preferred"},
	"derived-free-buffers-cached": {Canonical: "derived-free-buffers-cached", Trust: "derived"},
	"meminfo-derived":             {Canonical: "derived-free-buffers-cached", Trust: "derived"},
	"calculated":                  {Canonical: "derived-free-buffers-cached", Trust: "derived"},
	"derived-total-minus-used":    {Canonical: "derived-total-minus-used", Trust: "derived", Fallback: true, DefaultFallbackReason: "derived-total-minus-used"},
	"meminfo-total-minus-used":    {Canonical: "derived-total-minus-used", Trust: "derived", Fallback: true, DefaultFallbackReason: "derived-total-minus-used"},
	"rrd-memavailable":            {Canonical: "rrd-memavailable", Trust: "fallback", Fallback: true, DefaultFallbackReason: "rrd-memavailable"},
	"rrd-available":               {Canonical: "rrd-memavailable", Trust: "fallback", Fallback: true, DefaultFallbackReason: "rrd-memavailable"},
	"rrd-memused":                 {Canonical: "rrd-memused", Trust: "fallback", Fallback: true, DefaultFallbackReason: "rrd-memused"},
	"rrd-data":                    {Canonical: "rrd-memused", Trust: "fallback", Fallback: true, DefaultFallbackReason: "rrd-memused"},
	"agent":                       {Canonical: "agent", Trust: "fallback", Fallback: true, DefaultFallbackReason: "host-agent-memory"},
	"status-mem":                  {Canonical: "status-mem", Trust: "fallback", Fallback: true, DefaultFallbackReason: "status-mem"},
	"status-freemem":              {Canonical: "status-freemem", Trust: "fallback", Fallback: true, DefaultFallbackReason: "status-freemem"},
	"status-unavailable":          {Canonical: "status-unavailable", Trust: "fallback", Fallback: true, DefaultFallbackReason: "status-unavailable"},
	"cluster-resources":           {Canonical: "cluster-resources", Trust: "fallback", Fallback: true, DefaultFallbackReason: "cluster-resources"},
	"listing-mem":                 {Canonical: "cluster-resources", Trust: "fallback", Fallback: true, DefaultFallbackReason: "cluster-resources"},
	"listing":                     {Canonical: "cluster-resources", Trust: "fallback", Fallback: true, DefaultFallbackReason: "cluster-resources"},
	"powered-off":                 {Canonical: "powered-off", Trust: "fallback"},
}

func DescribeMemorySource(source string) MemorySourceDescriptor {
	key := strings.ToLower(strings.TrimSpace(source))
	if desc, ok := memorySourceCatalog[key]; ok {
		return desc
	}
	return MemorySourceDescriptor{
		Canonical: key,
		Trust:     "fallback",
		Fallback:  false,
	}
}

func CanonicalMemorySource(source string) string {
	return DescribeMemorySource(source).Canonical
}

func MemorySourceTrust(source string) string {
	return DescribeMemorySource(source).Trust
}

func MemorySourceIsFallback(source string) bool {
	return DescribeMemorySource(source).Fallback
}

func MemorySourceFallbackReason(source string) string {
	return DescribeMemorySource(source).DefaultFallbackReason
}
