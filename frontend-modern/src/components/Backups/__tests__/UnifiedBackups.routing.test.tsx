import { render, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import UnifiedBackups from '@/components/Backups/UnifiedBackups';

let mockLocationSearch = '';
const navigateSpy = vi.fn();

type BackupsFilterMock = {
  setSearch: (value: string) => void;
  setViewMode: (value: string) => void;
  setGroupBy: (value: 'date' | 'guest') => void;
};

let lastBackupsFilter: BackupsFilterMock | undefined;

vi.mock('@solidjs/router', async () => {
  const actual = await vi.importActual<typeof import('@solidjs/router')>('@solidjs/router');
  return {
    ...actual,
    useLocation: () => ({ pathname: '/backups', search: mockLocationSearch }),
    useNavigate: () => navigateSpy,
  };
});

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    state: {
      temperatureMonitoringEnabled: false,
      nodes: [
        {
          id: 'cluster-main-pve1',
          instance: 'cluster-main',
          name: 'pve1',
          type: 'node',
          status: 'online',
        },
      ],
      pbs: [],
      pveBackups: {
        backupTasks: [],
        storageBackups: [],
        guestSnapshots: [],
      },
      pbsBackups: [],
      pmgBackups: [],
      backups: {
        pve: {
          backupTasks: [],
          storageBackups: [],
          guestSnapshots: [],
        },
        pbs: [],
        pmg: [],
      },
      storage: [],
      vms: [],
      containers: [],
    },
    connected: () => true,
    initialDataReceived: () => true,
  }),
}));

vi.mock('@/components/shared/UnifiedNodeSelector', () => ({
  UnifiedNodeSelector: () => <div data-testid="backups-node-selector">node-selector</div>,
}));

vi.mock('@/components/Backups/BackupsFilter', () => ({
  BackupsFilter: (props: BackupsFilterMock) => {
    lastBackupsFilter = props;
    return <div data-testid="backups-filter">filter</div>;
  },
}));

vi.mock('@/components/Dashboard/MetricBar', () => ({
  MetricBar: () => <div data-testid="metric-bar">bar</div>,
}));

vi.mock('@/components/shared/SectionHeader', () => ({
  SectionHeader: () => <div data-testid="section-header">section</div>,
}));

vi.mock('@/components/shared/Tooltip', () => ({
  showTooltip: vi.fn(),
  hideTooltip: vi.fn(),
}));

/**
 * Legacy UnifiedBackups routing contract tests (compatibility-only).
 *
 * As of SB5-02, UnifiedBackups is no longer routed by App.tsx.
 * These tests document the legacy shell's internal routing behavior
 * for SB5-05 deletion readiness verification.
 *
 * @deprecated Scheduled for removal with UnifiedBackups.tsx in SB5-05.
 */
describe('UnifiedBackups routing contract', () => {
  beforeEach(() => {
    mockLocationSearch = '';
    navigateSpy.mockReset();
    lastBackupsFilter = undefined;
  });

  it('canonicalizes legacy search query params to q', async () => {
    mockLocationSearch = '?search=vm-101&migrated=1&from=hosts';

    render(() => <UnifiedBackups />);

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('q')).toBe('vm-101');
    expect(params.get('search')).toBeNull();
    expect(params.get('migrated')).toBe('1');
    expect(params.get('from')).toBe('hosts');
    expect(options?.replace).toBe(true);
  });

  it('syncs backups filter state into URL query params', async () => {
    render(() => <UnifiedBackups />);

    await waitFor(() => {
      expect(lastBackupsFilter).toBeDefined();
    });

    lastBackupsFilter!.setViewMode('pbs');
    lastBackupsFilter!.setGroupBy('guest');
    lastBackupsFilter!.setSearch('node:pve1');

    await waitFor(() => {
      expect(navigateSpy).toHaveBeenCalled();
    });

    const [path, options] = navigateSpy.mock.calls.at(-1) as [string, { replace?: boolean }];
    const params = new URLSearchParams(path.split('?')[1] || '');
    expect(params.get('source')).toBe('pbs');
    expect(params.get('backupType')).toBe('remote');
    expect(params.get('group')).toBe('guest');
    expect(params.get('q')).toBe('node:pve1');
    expect(options?.replace).toBe(true);
  });
});
