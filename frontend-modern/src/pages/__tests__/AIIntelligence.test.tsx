import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

import { AIIntelligence } from '../AIIntelligence';

const { findingsPanelState, runHistoryState } = vi.hoisted(() => ({
  findingsPanelState: {
    latestProps: null as {
      filterOverride?: string;
      filterFindingIds?: string[];
      scopeResourceIds?: string[];
      scopeResourceTypes?: string[];
      showScopeWarnings?: boolean;
    } | null,
  },
  runHistoryState: {
    selection: null as Record<string, unknown> | null,
  },
}));

const getPatrolStatusMock = vi.fn();
const getPatrolAutonomySettingsMock = vi.fn();
const updatePatrolAutonomySettingsMock = vi.fn();
const triggerPatrolRunMock = vi.fn();
const getPatrolRunHistoryMock = vi.fn();
const apiFetchJSONMock = vi.fn();
const hasFeatureMock = vi.fn();
const licenseStatusMock = vi.fn();
const loadLicenseStatusMock = vi.fn();
const startProTrialMock = vi.fn();
const getUpgradeActionUrlOrFallbackMock = vi.fn();
const trackPaywallViewedMock = vi.fn();
const trackUpgradeClickedMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();

vi.mock('@/api/patrol', () => ({
  getPatrolStatus: (...args: unknown[]) => getPatrolStatusMock(...args),
  getPatrolAutonomySettings: (...args: unknown[]) => getPatrolAutonomySettingsMock(...args),
  updatePatrolAutonomySettings: (...args: unknown[]) => updatePatrolAutonomySettingsMock(...args),
  triggerPatrolRun: (...args: unknown[]) => triggerPatrolRunMock(...args),
  getPatrolRunHistory: (...args: unknown[]) => getPatrolRunHistoryMock(...args),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  licenseStatus: (...args: unknown[]) => licenseStatusMock(...args),
  loadLicenseStatus: (...args: unknown[]) => loadLicenseStatusMock(...args),
  startProTrial: (...args: unknown[]) => startProTrialMock(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => getUpgradeActionUrlOrFallbackMock(...args),
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: (...args: unknown[]) => trackPaywallViewedMock(...args),
  trackUpgradeClicked: (...args: unknown[]) => trackUpgradeClickedMock(...args),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/stores/aiIntelligence', () => ({
  aiIntelligenceStore: {
    findings: [],
    loadFindings: vi.fn().mockResolvedValue(undefined),
    loadCircuitBreakerStatus: vi.fn().mockResolvedValue(undefined),
    loadPendingApprovals: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock('@/components/AI/FindingsPanel', () => ({
  FindingsPanel: (props: Record<string, unknown>) => {
    findingsPanelState.latestProps = {
      filterOverride:
        typeof props.filterOverride === 'string' ? props.filterOverride : undefined,
      filterFindingIds: Array.isArray(props.filterFindingIds)
        ? [...(props.filterFindingIds as string[])]
        : undefined,
      scopeResourceIds: Array.isArray(props.scopeResourceIds)
        ? [...(props.scopeResourceIds as string[])]
        : undefined,
      scopeResourceTypes: Array.isArray(props.scopeResourceTypes)
        ? [...(props.scopeResourceTypes as string[])]
        : undefined,
      showScopeWarnings:
        typeof props.showScopeWarnings === 'boolean' ? props.showScopeWarnings : undefined,
    };
    return <div data-testid="findings-panel" />;
  },
}));

vi.mock('@/components/patrol', () => ({
  ApprovalBanner: () => <div data-testid="approval-banner" />,
  PatrolStatusBar: () => <div data-testid="patrol-status-bar" />,
  RunHistoryPanel: (props: {
    onSelectRun?: (run: Record<string, unknown> | null) => void;
  }) => (
    <div data-testid="run-history-panel">
      <button type="button" onClick={() => props.onSelectRun?.(runHistoryState.selection)}>
        Select mocked run
      </button>
    </div>
  ),
  CountdownTimer: () => <div data-testid="countdown-timer" />,
}));

vi.mock('@/hooks/usePatrolStream', () => ({
  usePatrolStream: () => ({
    isStreaming: () => false,
    phase: () => '',
    currentTool: () => '',
    tokens: () => 0,
  }),
}));

vi.mock('@/components/shared/PageHeader', () => ({
  PageHeader: (props: {
    title?: string;
    actions?: unknown;
  }) => (
    <div>
      <h1>{props.title}</h1>
      <div>{props.actions as any}</div>
    </div>
  ),
}));

vi.mock('@/components/shared/Toggle', () => ({
  TogglePrimitive: (props: {
    checked?: boolean;
    disabled?: boolean;
    onToggle?: (value: boolean) => void;
    ariaLabel?: string;
  }) => (
    <button
      type="button"
      aria-label={props.ariaLabel}
      aria-pressed={props.checked}
      disabled={props.disabled}
      onClick={() => props.onToggle?.(!props.checked)}
    />
  ),
  Toggle: (props: {
    checked?: boolean;
    disabled?: boolean;
    onChange?: (event: Event & { currentTarget: HTMLInputElement }) => void;
  }) => (
    <input
      type="checkbox"
      checked={props.checked}
      disabled={props.disabled}
      onChange={(event) => props.onChange?.(event as Event & { currentTarget: HTMLInputElement })}
    />
  ),
}));

vi.mock('@/components/Brand/PulsePatrolLogo', () => ({
  PulsePatrolLogo: () => <div data-testid="pulse-patrol-logo" />,
}));

const defaultPatrolStatus = (overrides: Record<string, unknown> = {}) => ({
  running: false,
  using_quickstart: false,
  quickstart_credits_total: 0,
  quickstart_credits_remaining: 0,
  license_required: false,
  blocked_reason: '',
  blocked_at: '',
  ...overrides,
});

const defaultAISettings = {
  patrol_enabled: true,
  patrol_interval_minutes: 360,
  patrol_model: '',
  model: '',
  alert_triggered_analysis: false,
  patrol_event_triggers_enabled: true,
  patrol_auto_fix: false,
  auto_fix_model: '',
};

describe('AIIntelligence entitlement gating', () => {
  beforeEach(() => {
    getPatrolStatusMock.mockReset();
    getPatrolAutonomySettingsMock.mockReset();
    updatePatrolAutonomySettingsMock.mockReset();
    triggerPatrolRunMock.mockReset();
    getPatrolRunHistoryMock.mockReset();
    apiFetchJSONMock.mockReset();
    hasFeatureMock.mockReset();
    licenseStatusMock.mockReset();
    loadLicenseStatusMock.mockReset();
    startProTrialMock.mockReset();
    getUpgradeActionUrlOrFallbackMock.mockReset();
    trackPaywallViewedMock.mockReset();
    trackUpgradeClickedMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    findingsPanelState.latestProps = null;
    runHistoryState.selection = null;

    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus());
    getPatrolAutonomySettingsMock.mockResolvedValue({
      autonomy_level: 'monitor',
      full_mode_unlocked: false,
      investigation_budget: 15,
      investigation_timeout_sec: 300,
    });
    updatePatrolAutonomySettingsMock.mockResolvedValue({
      settings: {
        autonomy_level: 'monitor',
        full_mode_unlocked: false,
      },
    });
    triggerPatrolRunMock.mockResolvedValue(undefined);
    getPatrolRunHistoryMock.mockResolvedValue([]);
    apiFetchJSONMock.mockImplementation(async (path: string) => {
      if (path === '/api/ai/models') {
        return { models: [] };
      }
      if (path === '/api/settings/ai') {
        return defaultAISettings;
      }
      if (path === '/api/settings/ai/update') {
        return defaultAISettings;
      }
      return {};
    });
    hasFeatureMock.mockImplementation((feature: string) => !['ai_alerts', 'ai_autofix'].includes(feature));
    licenseStatusMock.mockReturnValue({ subscription_state: 'expired' });
    loadLicenseStatusMock.mockResolvedValue(undefined);
    startProTrialMock.mockResolvedValue({ outcome: 'started' });
    getUpgradeActionUrlOrFallbackMock.mockImplementation(
      (feature?: string) => `/upgrade${feature ? `?feature=${feature}` : ''}`,
    );
  });

  afterEach(() => {
    cleanup();
  });

  it('locks paid patrol controls and shows upgrade paths for free entitlements', async () => {
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        license_required: true,
      }),
    );

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(loadLicenseStatusMock).toHaveBeenCalled();
      expect(getPatrolStatusMock).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Configure Patrol' }));

    await screen.findByRole('button', { name: 'Investigate' });

    expect(screen.getByRole('button', { name: 'Investigate' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Auto-fix' })).toBeDisabled();
    expect(
      screen
        .getAllByRole('link', { name: 'Upgrade to Pro' })
        .some((link) => link.getAttribute('href') === '/upgrade?feature=ai_autofix'),
    ).toBe(true);
    expect(screen.getByRole('link', { name: 'Upgrade' })).toHaveAttribute(
      'href',
      '/upgrade?feature=ai_alerts',
    );

    await waitFor(() => {
      expect(trackPaywallViewedMock).toHaveBeenCalledWith('ai_autofix', 'ai_intelligence');
      expect(trackPaywallViewedMock).toHaveBeenCalledWith('ai_alerts', 'ai_intelligence');
      expect(trackPaywallViewedMock).toHaveBeenCalledWith('ai_autofix', 'ai_intelligence_banner');
    });
  });

  it('unlocks paid patrol controls when the entitlement grants the features', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(loadLicenseStatusMock).toHaveBeenCalled();
      expect(getPatrolStatusMock).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Configure Patrol' }));

    await screen.findByRole('button', { name: 'Investigate' });

    expect(screen.getByRole('button', { name: 'Investigate' })).not.toBeDisabled();
    expect(screen.getByRole('button', { name: 'Auto-fix' })).not.toBeDisabled();
    expect(screen.queryByRole('link', { name: 'Upgrade to Pro' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade' })).not.toBeInTheDocument();
    expect(trackPaywallViewedMock).not.toHaveBeenCalled();
  });

  it('treats a selected zero-finding run as an empty snapshot and uses effective scope ids', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    runHistoryState.selection = {
      id: 'run-empty',
      started_at: '2026-03-12T10:00:00Z',
      completed_at: '2026-03-12T10:01:00Z',
      duration_ms: 60000,
      type: 'scoped',
      trigger_reason: 'alert_fired',
      scope_resource_ids: ['seed-resource'],
      effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
      scope_resource_types: ['vm'],
      resources_checked: 2,
      nodes_checked: 0,
      guests_checked: 2,
      docker_checked: 0,
      storage_checked: 0,
      hosts_checked: 0,
      pbs_checked: 0,
      pmg_checked: 0,
      kubernetes_checked: 0,
      new_findings: 0,
      existing_findings: 0,
      rejected_findings: 0,
      resolved_findings: 0,
      auto_fix_count: 0,
      findings_summary: 'All healthy',
      finding_ids: [],
      error_count: 0,
      status: 'healthy',
      tool_call_count: 0,
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(getPatrolStatusMock).toHaveBeenCalled();
      expect(findingsPanelState.latestProps).not.toBeNull();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Run History' }));
    fireEvent.click(screen.getByRole('button', { name: 'Select mocked run' }));
    fireEvent.click(screen.getByRole('button', { name: 'Findings' }));

    await waitFor(() => {
      expect(screen.getByText(/Filtered to run/i)).toBeInTheDocument();
    });

    expect(findingsPanelState.latestProps).toMatchObject({
      filterOverride: 'all',
      filterFindingIds: [],
      scopeResourceIds: ['expanded-a', 'expanded-b'],
      scopeResourceTypes: ['vm'],
      showScopeWarnings: true,
    });
  });

  it('keeps an explicit empty effective scope instead of falling back to seed scope ids', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    runHistoryState.selection = {
      id: 'run-empty-effective-scope',
      started_at: '2026-03-12T10:05:00Z',
      completed_at: '2026-03-12T10:06:00Z',
      duration_ms: 60000,
      type: 'scoped',
      trigger_reason: 'alert_fired',
      scope_resource_ids: ['seed-resource'],
      effective_scope_resource_ids: [],
      scope_resource_types: ['vm'],
      resources_checked: 0,
      nodes_checked: 0,
      guests_checked: 0,
      docker_checked: 0,
      storage_checked: 0,
      hosts_checked: 0,
      pbs_checked: 0,
      pmg_checked: 0,
      kubernetes_checked: 0,
      new_findings: 0,
      existing_findings: 0,
      rejected_findings: 0,
      resolved_findings: 0,
      auto_fix_count: 0,
      findings_summary: 'Nothing matched',
      finding_ids: [],
      error_count: 0,
      status: 'healthy',
      tool_call_count: 0,
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(getPatrolStatusMock).toHaveBeenCalled();
      expect(findingsPanelState.latestProps).not.toBeNull();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Run History' }));
    fireEvent.click(screen.getByRole('button', { name: 'Select mocked run' }));
    fireEvent.click(screen.getByRole('button', { name: 'Findings' }));

    await waitFor(() => {
      expect(screen.getByText(/Filtered to run/i)).toBeInTheDocument();
    });

    expect(findingsPanelState.latestProps).toMatchObject({
      filterOverride: 'all',
      filterFindingIds: [],
      scopeResourceIds: [],
      scopeResourceTypes: ['vm'],
      showScopeWarnings: true,
    });
  });
});
