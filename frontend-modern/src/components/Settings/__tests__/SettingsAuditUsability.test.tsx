import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { getSettingsNavGroups } from '../settingsNavCatalog';
import { resetSettingsScrollPosition } from '../Settings';
import { AvailabilityTargetSlot } from '../ConnectionEditor/CredentialSlots/AvailabilityTargetSlot';
import AuditLogPanel from '../AuditLogPanel';
import { DiagnosticsPanel } from '../DiagnosticsPanel';
import { InfrastructureSourceManager } from '../InfrastructureSourceManager';
import type { Connection } from '@/api/connections';
import { primaryRowProblem, type InfrastructureSystemRow } from '../connectionsTableModel';

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ resources: () => [] }),
}));

vi.mock('@/stores/license', () => ({
  getRuntimeCapabilityBlock: () => undefined,
  hasFeature: () => false,
  isHostedModeEnabled: () => false,
  loadRuntimeCapabilities: vi.fn().mockResolvedValue(undefined),
  runtimeCapabilitiesLoaded: () => true,
}));

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: () => ({
    href: '/settings/pulse-intelligence/billing/plan',
    external: false,
  }),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesCommercialSurfaces: () => false,
  presentationPolicyHidesUpgradePrompts: () => false,
}));

vi.mock('../useDiagnosticsPanelState', () => ({
  useDiagnosticsPanelState: () => ({
    diagnosticsData: () => ({
      version: '6.8.0',
      runtime: 'go',
      uptime: 1285.723469934999912,
      nodes: [],
      pbs: [],
      system: {
        os: 'linux',
        arch: 'amd64',
        goVersion: 'go1.25',
        numCPU: 4,
        numGoroutine: 12,
        memoryMB: 64,
      },
      errors: [],
    }),
    error: () => '',
    exportDiagnostics: vi.fn(),
    exportLoading: () => false,
    loading: () => false,
    notice: () => '',
    runDiagnostics: vi.fn(),
  }),
}));

const makeRow = (
  name: string,
  ownerType: Connection['type'],
  address: string,
): InfrastructureSystemRow => {
  const connection: Connection = {
    id: `${ownerType}:${name}`,
    type: ownerType,
    name,
    address,
    state: 'active',
    stateReason: '',
    enabled: true,
    surfaces: ['host'],
    scope: { host: true },
    lastSeen: '2026-11-23T19:00:00Z',
    lastError: null,
    source: ownerType === 'agent' ? 'agent' : 'manual',
    capabilities: { supportsPause: false, supportsScope: false, supportsTest: false },
  };
  return {
    id: connection.id,
    ownerType,
    name,
    subtitle: ownerType,
    source: ownerType === 'agent' ? 'agent' : 'api',
    host: address,
    coverageLabels: ['Host telemetry'],
    statusLabel: 'Active',
    statusClassName: 'bg-green-100 text-green-800',
    agentUpdateCount: 0,
    lastActivityText: '5s ago',
    fleetSignals: [],
    fleetHighlights: [],
    problem: primaryRowProblem([]),
    enabled: true,
    canEdit: true,
    canPause: false,
    canRemove: true,
    isAgent: ownerType === 'agent',
    isCluster: false,
    attachedConnections: [],
    members: [],
    connection,
  };
};

describe('Settings usability audit fixes', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('matches common search synonyms in the settings navigation catalog', () => {
    const items = getSettingsNavGroups('en').flatMap((group) => group.items);
    const matchingIds = (query: string) => {
      const normalized = query.toLowerCase();
      return items
        .filter(
          (item) =>
            item.label.toLowerCase().includes(normalized) ||
            (item.keywords ?? []).some((keyword) => keyword.toLowerCase().includes(normalized)),
        )
        .map((item) => item.id);
    };

    expect(matchingIds('sso')).toContain('security-sso');
    expect(matchingIds('oidc')).toContain('security-sso');
    expect(matchingIds('saml')).toContain('security-sso');
    expect(matchingIds('dark')).toContain('system-general');
    expect(matchingIds('theme')).toContain('system-general');
    expect(matchingIds('branding')).toContain('system-general');
    expect(matchingIds('proxmox')).toContain('infrastructure-systems');
    expect(matchingIds('docker')).toContain('infrastructure-systems');
    expect(matchingIds('kubernetes')).toContain('infrastructure-systems');
    expect(matchingIds('token')).toContain('api');
    expect(matchingIds('password')).toContain('security-auth');
    expect(matchingIds('backup')).toContain('system-recovery');
    expect(matchingIds('restore')).toContain('system-recovery');
    expect(matchingIds('logs')).toContain('support-logs');
    expect(matchingIds('diagnostics')).toContain('support-diagnostics');
    expect(matchingIds('patrol')).toContain('system-ai-patrol');
    expect(matchingIds('webhook')).toContain('security-webhooks');
  });

  it('resets .app-scroll-shell scrollTop and window scroll position when switching Settings tabs', () => {
    const scrollToSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined);
    const shell = document.createElement('div');
    shell.className = 'app-scroll-shell';
    shell.scrollTop = 640;
    document.body.appendChild(shell);
    try {
      resetSettingsScrollPosition();
      expect(shell.scrollTop).toBe(0);
      expect(scrollToSpy).toHaveBeenCalledWith({ top: 0, behavior: 'auto' });
    } finally {
      shell.remove();
    }
  });

  it('places the AvailabilityTargetSlot action footer outside the scrollable form container', () => {
    window.history.replaceState({}, '', '/');
    const { container } = render(() => (
      <Router>
        <Route
          path="/"
          component={() => <AvailabilityTargetSlot onCancel={vi.fn()} onSaved={vi.fn()} />}
        />
      </Router>
    ));

    const footer = container.querySelector('[data-availability-modal-footer]');
    const scrollBody = container.querySelector('.overflow-y-auto');
    expect(footer).not.toBeNull();
    expect(scrollBody).not.toBeNull();
    expect(footer?.closest('.overflow-y-auto')).toBeNull();
    expect(scrollBody?.contains(footer)).toBe(false);
    expect(footer).toHaveTextContent('Cancel');
    expect(footer).toHaveTextContent('Test probe');
    expect(footer).toHaveTextContent('Add service/device check');
  });

  it('renders integer uptime and styled documentation link in DiagnosticsPanel', () => {
    render(() => <DiagnosticsPanel canRead={true} />);

    expect(screen.getByText(/Uptime: 21m 25s/)).toBeInTheDocument();
    const link = screen.getByRole('link', { name: 'Safe diagnostics and sharing' });
    expect(link).toHaveClass('text-blue-600', 'dark:text-blue-400', 'hover:underline');
    expect(link).not.toHaveClass('text-link');
  });

  it('hides disabled runtime Audit Log toolbar buttons when gated behind the Pro paywall', () => {
    window.history.replaceState({}, '', '/settings/security-audit');
    render(() => (
      <Router>
        <Route path="/settings/security-audit" component={() => <AuditLogPanel />} />
      </Router>
    ));

    expect(screen.queryByRole('button', { name: 'Refresh' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Verify All' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Resume/ })).not.toBeInTheDocument();
  });

  it('filters connected systems in Infrastructure settings and keeps Discovery above the table', () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1440 });
    const rows = [
      makeRow('pve-core-01', 'pve', 'https://10.0.0.10:8006'),
      makeRow('truenas-nas-01', 'truenas', 'https://10.0.0.20'),
      makeRow('docker-edge-01', 'agent', '10.0.0.30'),
    ];

    render(() => (
      <InfrastructureSourceManager
        rows={() => rows}
        discoveredNodes={() => []}
        discoveryEnabled={true}
        discoveryScanStatus={() => ({ scanning: false })}
        readOnly={false}
        onRunDiscovery={vi.fn()}
        onOpenDiscoverySettings={vi.fn()}
      />
    ));

    const discovery = screen.getByRole('region', { name: 'Discover Proxmox systems' });
    const table = screen.getByRole('table');
    expect(
      discovery.compareDocumentPosition(table) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    const search = screen.getByPlaceholderText(
      'Filter connected systems by name, address, platform, or status...',
    );
    fireEvent.input(search, { target: { value: 'truenas' } });

    expect(screen.getByText('truenas-nas-01')).toBeInTheDocument();
    expect(screen.queryByText('pve-core-01')).not.toBeInTheDocument();
    expect(screen.queryByText('docker-edge-01')).not.toBeInTheDocument();
    expect(screen.getByText('Showing 1 of 3 systems')).toBeInTheDocument();

    fireEvent.input(search, { target: { value: 'does-not-exist' } });
    expect(screen.getByText('No matching connected systems')).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: 'Clear filter' })[0]);
    expect(screen.getByText('pve-core-01')).toBeInTheDocument();
    expect(screen.getByText('truenas-nas-01')).toBeInTheDocument();
    expect(screen.getByText('docker-edge-01')).toBeInTheDocument();
  });
});
