package discovery

import "context"

const (
	ProductPVE = productPVE
	ProductPMG = productPMG
	ProductPBS = productPBS
)

func (s *Scanner) ProbeProxmoxService(ctx context.Context, ip string, port int) *ProxmoxProbeResult {
	return s.probeProxmoxService(ctx, ip, port)
}

func (s *Scanner) ProbeAPIEndpoint(ctx context.Context, address, endpoint string) EndpointProbeFinding {
	return s.probeAPIEndpoint(ctx, address, endpoint)
}
