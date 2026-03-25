import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { AIIntelligence } from '../AIIntelligence';

const { findingsPanelState, runHistoryState, intelligenceState } = vi.hoisted(() => ({
  findingsPanelState: {
    latestProps: null as {
      filterOverride?: string;
      filterFindingIds?: string[];
      scopeResourceIds?: string[];
      scopeResourceTypes?: string[];
      showScopeWarnings?: boolean;
      nextPatrolAt?: string;
      lastPatrolAt?: string;
      lastPatrolLabel?: string;
      patrolIntervalMs?: number;
    } | null,
  },
  runHistoryState: {
    selection: null as Record<string, unknown> | null,
  },
  intelligenceState: {
    findings: [] as Array<Record<string, unknown>>,
    summary: null as {
      timestamp: string;
      overall_health: {
        score: number;
        grade: 'A' | 'B' | 'C' | 'D' | 'F';
        trend: 'improving' | 'stable' | 'declining';
        factors: Array<Record<string, unknown>>;
        prediction: string;
      };
      findings_count: {
        critical: number;
        warning: number;
        watch: number;
        info: number;
        total: number;
      };
      predictions_count: number;
      recent_changes_count: number;
      recent_changes: Array<{
        id: string;
        observedAt: string;
        resourceId: string;
        kind: string;
        sourceType: string;
        sourceAdapter?: string;
        confidence: string;
        reason?: string;
        relatedResources?: string[];
      }>;
      policy_posture?: {
        total_resources: number;
        sensitivity_counts: Record<string, number>;
        routing_counts: Record<string, number>;
        redaction_counts?: Record<string, number>;
      };
      learning: {
        resources_with_knowledge: number;
        total_notes: number;
        resources_with_baselines: number;
        patterns_detected: number;
        correlations_learned: number;
        incidents_tracked: number;
      };
    } | null,
    correlations: null as {
      correlations: Array<{
        source_id: string;
        source_name: string;
        source_type: string;
        target_id: string;
        target_name: string;
        target_type: string;
        event_pattern: string;
        occurrences: number;
        avg_delay: number | string;
        confidence: number;
        last_seen: string;
        description?: string;
      }>;
      count: number;
    } | null,
  },
}));

const getCorrelationsMock = vi.fn();
const [correlationsState, setCorrelationsState] =
  createSignal<(typeof intelligenceState)['correlations']>(null);
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

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getCorrelations: (...args: unknown[]) => getCorrelationsMock(...args),
  },
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

vi.mock('@/stores/aiIntelligence', () => {
  const store = {
    loadFindings: vi.fn().mockResolvedValue(undefined),
    loadIntelligenceSummary: vi.fn().mockResolvedValue(undefined),
    loadCircuitBreakerStatus: vi.fn().mockResolvedValue(undefined),
    loadPendingApprovals: vi.fn().mockResolvedValue(undefined),
    loadCorrelations: vi.fn().mockImplementation(async () => {
      const response = await getCorrelationsMock();
      intelligenceState.correlations = response;
      setCorrelationsState(response);
      return response;
    }),
    loadDashboardData: vi.fn().mockImplementation(async () => {
      await Promise.all([
        store.loadFindings(),
        store.loadIntelligenceSummary(),
        store.loadCircuitBreakerStatus(),
        store.loadPendingApprovals(),
        store.loadCorrelations(),
      ]);
    }),
    get findings() {
      return intelligenceState.findings;
    },
    get intelligenceSummary() {
      return intelligenceState.summary;
    },
    get correlations() {
      return correlationsState();
    },
  };

  return {
    aiIntelligenceStore: store,
  };
});

vi.mock('@/components/AI/FindingsPanel', () => ({
  FindingsPanel: (props: Record<string, unknown>) => {
    findingsPanelState.latestProps = {
      filterOverride: typeof props.filterOverride === 'string' ? props.filterOverride : undefined,
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
      nextPatrolAt: typeof props.nextPatrolAt === 'string' ? props.nextPatrolAt : undefined,
      lastPatrolAt: typeof props.lastPatrolAt === 'string' ? props.lastPatrolAt : undefined,
      lastPatrolLabel:
        typeof props.lastPatrolLabel === 'string' ? props.lastPatrolLabel : undefined,
      patrolIntervalMs:
        typeof props.patrolIntervalMs === 'number' ? props.patrolIntervalMs : undefined,
    };
    return <div data-testid="findings-panel" />;
  },
}));

vi.mock('@/components/patrol', () => ({
  ApprovalBanner: () => <div data-testid="approval-banner" />,
  PatrolStatusBar: () => <div data-testid="patrol-status-bar" />,
  RunHistoryPanel: (props: { onSelectRun?: (run: Record<string, unknown> | null) => void }) => (
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
  PageHeader: (props: { title?: string; actions?: unknown }) => (
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
  runtime_state: 'active',
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
    intelligenceState.findings = [];
    intelligenceState.summary = null;
    intelligenceState.correlations = null;
    setCorrelationsState(null);
    getCorrelationsMock.mockReset();

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
    hasFeatureMock.mockImplementation(
      (feature: string) => !['ai_alerts', 'ai_autofix'].includes(feature),
    );
    licenseStatusMock.mockReturnValue({ subscription_state: 'expired' });
    loadLicenseStatusMock.mockResolvedValue(undefined);
    startProTrialMock.mockResolvedValue({ outcome: 'started' });
    getUpgradeActionUrlOrFallbackMock.mockImplementation(
      (feature?: string) => `/upgrade${feature ? `?feature=${feature}` : ''}`,
    );
    getCorrelationsMock.mockResolvedValue({
      correlations: [],
      count: 0,
    });
  });

  it('renders canonical learned correlations in the summary page through the correlation card', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    intelligenceState.summary = {
      timestamp: '2026-03-01T00:00:00Z',
      overall_health: {
        score: 91,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Stable',
      },
      findings_count: {
        critical: 0,
        warning: 0,
        watch: 0,
        info: 0,
        total: 0,
      },
      predictions_count: 0,
      recent_changes_count: 0,
      recent_changes: [],
      learning: {
        resources_with_knowledge: 0,
        total_notes: 0,
        resources_with_baselines: 0,
        patterns_detected: 0,
        correlations_learned: 1,
        incidents_tracked: 0,
      },
    };
    getCorrelationsMock.mockResolvedValue({
      correlations: [
        {
          source_id: 'storage-2',
          source_name: 'Storage 2',
          source_type: 'storage',
          target_id: 'vm-200',
          target_name: 'VM 200',
          target_type: 'vm',
          event_pattern: 'disk_full -> restart',
          occurrences: 2,
          avg_delay: '1m30s',
          confidence: 0.95,
          last_seen: '2026-03-01T00:05:00Z',
          description: 'Disk pressure often precedes restarts',
        },
        {
          source_id: 'storage-1',
          source_name: 'Storage 1',
          source_type: 'storage',
          target_id: 'vm-100',
          target_name: 'VM 100',
          target_type: 'vm',
          event_pattern: 'cpu_high -> restart',
          occurrences: 1,
          avg_delay: '3m',
          confidence: 0.5,
          last_seen: '2026-03-01T00:03:00Z',
          description: 'Lower-confidence backup pattern',
        },
      ],
      count: 2,
    });

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByText('Investigation context')).toBeInTheDocument();
    });

    expect(screen.queryByRole('heading', { name: 'Correlations' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Show context' }));
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Correlations' })).toBeInTheDocument();
    });

    expect(screen.getByText('2 total')).toBeInTheDocument();
    const storage2Link = screen.getByRole('link', {
      name: 'Open source resource Storage 2 in Infrastructure',
    });
    const storage1Link = screen.getByRole('link', {
      name: 'Open source resource Storage 1 in Infrastructure',
    });
    expect(storage2Link).toHaveAttribute('href', '/infrastructure?resource=storage-2');
    expect(
      screen.getByRole('link', { name: 'Open target resource VM 200 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=vm-200');
    expect(
      screen.getByRole('link', { name: 'Open target resource VM 100 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=vm-100');
    expect(
      storage2Link.compareDocumentPosition(storage1Link) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.getByText('Storage 1')).toBeInTheDocument();
    expect(screen.getByText('VM 100')).toBeInTheDocument();
    expect(screen.getByText('Cpu High → Restart')).toBeInTheDocument();
    expect(screen.getByText('Disk Full → Restart')).toBeInTheDocument();
    expect(screen.getByText(/2 occurrences .* 95% confidence/)).toBeInTheDocument();
    expect(screen.getByText(/1 occurrence .* 50% confidence/)).toBeInTheDocument();
    expect(screen.getByText('Disk pressure often precedes restarts')).toBeInTheDocument();
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

  it('renders the canonical intelligence summary card with recent changes', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    intelligenceState.summary = {
      timestamp: '2026-03-01T00:00:00Z',
      overall_health: {
        score: 91,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Stable',
      },
      findings_count: {
        critical: 1,
        warning: 2,
        watch: 0,
        info: 4,
        total: 7,
      },
      predictions_count: 3,
      recent_changes_count: 1,
      recent_changes: [
        {
          id: 'change-1',
          observedAt: '2026-03-01T00:00:00Z',
          resourceId: 'vm-100',
          kind: 'config_update',
          sourceType: 'pulse_diff',
          sourceAdapter: 'proxmox_adapter',
          confidence: 'high',
          reason: 'Updated guest configuration',
          relatedResources: ['agent-1'],
        },
      ],
      policy_posture: {
        total_resources: 4,
        sensitivity_counts: {
          public: 1,
          internal: 1,
          sensitive: 1,
          restricted: 1,
        },
        routing_counts: {
          'cloud-summary': 2,
          'local-first': 1,
          'local-only': 1,
        },
        redaction_counts: {
          hostname: 2,
          'ip-address': 1,
        },
      },
      learning: {
        resources_with_knowledge: 4,
        total_notes: 11,
        resources_with_baselines: 3,
        patterns_detected: 2,
        correlations_learned: 1,
        incidents_tracked: 5,
      },
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(getPatrolStatusMock).toHaveBeenCalled();
      expect(screen.getByText('Patrol assessment')).toBeInTheDocument();
      expect(screen.getByText('No active issues detected')).toBeInTheDocument();
    });

    const findingsPanel = screen.getByTestId('findings-panel');
    const contextHeading = screen.getByText('Investigation context');
    expect(
      Boolean(findingsPanel.compareDocumentPosition(contextHeading) & Node.DOCUMENT_POSITION_FOLLOWING),
    ).toBe(true);

    expect(screen.getByText(/Health A · 91\/100/)).toBeInTheDocument();
    expect(screen.getByText('Investigation context')).toBeInTheDocument();
    expect(screen.getByText('1 recent change · 4 governed resources')).toBeInTheDocument();
    expect(screen.queryByText('Policy posture')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Show context' }));

    await waitFor(() => {
      expect(screen.getByText('Policy posture')).toBeInTheDocument();
    });

    expect(screen.getByText('Config update: Updated guest configuration')).toBeInTheDocument();
    expect(screen.getByText('Updated guest configuration')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: 'Open resource vm-100 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=vm-100');
    expect(
      screen.getByRole('link', { name: 'Open related resource agent-1 in Infrastructure' }),
    ).toHaveAttribute('href', '/infrastructure?resource=agent-1');
    expect(screen.queryByText('Derived signal coverage')).not.toBeInTheDocument();
    expect(screen.queryByText(/baselined/)).not.toBeInTheDocument();
    expect(screen.getByText('Recent changes')).toBeInTheDocument();
    expect(screen.getByText('4 governed resources')).toBeInTheDocument();
    expect(screen.getByText('Public')).toBeInTheDocument();
    expect(screen.getByText('Internal')).toBeInTheDocument();
    expect(screen.getByText('Sensitive')).toBeInTheDocument();
    expect(screen.getByText('Restricted')).toBeInTheDocument();
    expect(screen.getByText('Cloud Summary')).toBeInTheDocument();
    expect(screen.getByText('Local First')).toBeInTheDocument();
    expect(screen.getByText('Local Only')).toBeInTheDocument();
    expect(screen.getByText('Hostname 2')).toBeInTheDocument();
    expect(screen.getByText('IP Address 1')).toBeInTheDocument();
  });

  it('does not present a healthy patrol summary when patrol is blocked on exhausted quickstart credits', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        runtime_state: 'blocked',
        using_quickstart: true,
        quickstart_credits_total: 25,
        quickstart_credits_remaining: 0,
        blocked_reason:
          'Quickstart credits exhausted. Connect your API key to continue using AI Patrol.',
        last_patrol_at: '2026-03-12T09:57:00Z',
      }),
    );
    intelligenceState.summary = {
      timestamp: '2026-03-12T10:00:00Z',
      overall_health: {
        score: 100,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Infrastructure is healthy with no significant issues detected.',
      },
      findings_count: {
        critical: 0,
        warning: 0,
        watch: 0,
        info: 0,
        total: 0,
      },
      predictions_count: 0,
      recent_changes_count: 0,
      learning: {
        resources_with_knowledge: 0,
        total_notes: 0,
        resources_with_baselines: 0,
        patterns_detected: 0,
        correlations_learned: 0,
        incidents_tracked: 0,
      },
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(getPatrolStatusMock).toHaveBeenCalled();
      expect(screen.getAllByText('Patrol Paused')).toHaveLength(2);
    });

    expect(screen.getAllByText('Patrol paused').length).toBeGreaterThan(0);
    expect(
      screen.getAllByText(
        'Quickstart credits exhausted. Connect your API key to continue using AI Patrol.',
      ).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText(/Health A · 100\/100/)).not.toBeInTheDocument();
  });

  it('does not show the exhausted quickstart chip when patrol is active on a configured provider path', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        runtime_state: 'active',
        using_quickstart: false,
        quickstart_credits_total: 25,
        quickstart_credits_remaining: 0,
        last_patrol_at: '2026-03-12T09:57:00Z',
      }),
    );
    intelligenceState.summary = {
      timestamp: '2026-03-12T10:00:00Z',
      overall_health: {
        score: 100,
        grade: 'A',
        trend: 'stable',
        factors: [],
        prediction: 'Infrastructure is healthy with no significant issues detected.',
      },
      findings_count: {
        critical: 0,
        warning: 0,
        watch: 0,
        info: 0,
        total: 0,
      },
      predictions_count: 0,
      recent_changes_count: 0,
      learning: {
        resources_with_knowledge: 0,
        total_notes: 0,
        resources_with_baselines: 0,
        patterns_detected: 0,
        correlations_learned: 0,
        incidents_tracked: 0,
      },
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(getPatrolStatusMock).toHaveBeenCalled();
      expect(screen.getByText('Patrol enabled')).toBeInTheDocument();
    });

    expect(screen.queryByText('Credits exhausted — connect API key')).not.toBeInTheDocument();
    expect(screen.getByText(/Health A · 100\/100/)).toBeInTheDocument();
  });

  it('surfaces coverage incomplete as the primary patrol assessment instead of no-issues copy', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        runtime_state: 'active',
        last_patrol_at: '2026-03-12T09:57:00Z',
      }),
    );
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-scoped',
        started_at: '2026-03-12T09:56:00Z',
        completed_at: '2026-03-12T09:57:00Z',
        duration_ms: 60000,
        type: 'scoped',
        trigger_reason: 'alert_fired',
        scope_resource_ids: ['vm-100'],
        effective_scope_resource_ids: ['vm-100'],
        scope_resource_types: ['vm'],
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 1,
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
        findings_summary: '',
        finding_ids: [],
        error_count: 1,
        status: 'error',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);
    intelligenceState.summary = {
      timestamp: '2026-03-12T10:00:00Z',
      overall_health: {
        score: 70,
        grade: 'C',
        trend: 'stable',
        factors: [
          {
            name: 'Patrol coverage incomplete',
            impact: -0.35,
            description:
              'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
            category: 'coverage',
          },
        ],
        prediction:
          'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
      },
      findings_count: {
        critical: 0,
        warning: 0,
        watch: 0,
        info: 0,
        total: 0,
      },
      predictions_count: 0,
      recent_changes_count: 0,
      learning: {
        resources_with_knowledge: 0,
        total_notes: 0,
        resources_with_baselines: 0,
        patterns_detected: 0,
        correlations_learned: 0,
        incidents_tracked: 0,
      },
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getAllByText('Coverage incomplete').length).toBeGreaterThan(0);
      expect(screen.getByText('No recent full patrol')).toBeInTheDocument();
      expect(screen.getAllByText(/Last activity/i)).toHaveLength(1);
      expect(screen.getAllByText('Active findings')).toHaveLength(1);
      expect(screen.getAllByText('Warnings')).toHaveLength(1);
      expect(screen.getAllByText('Critical')).toHaveLength(1);
      expect(findingsPanelState.latestProps).not.toBeNull();
    });

    expect(screen.queryByText('No issues found')).not.toBeInTheDocument();
    expect(screen.queryByText(/Last patrol/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Last:/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Partial verification')).not.toBeInTheDocument();
    expect(
      screen.getAllByText(
        'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
      ),
    ).toHaveLength(1);
    expect(findingsPanelState.latestProps?.nextPatrolAt).toBeUndefined();
    expect(findingsPanelState.latestProps?.lastPatrolAt).toBeUndefined();
    expect(findingsPanelState.latestProps?.lastPatrolLabel).toBeUndefined();
    expect(findingsPanelState.latestProps?.patrolIntervalMs).toBeUndefined();
  });

  it('describes both findings and incomplete coverage when active issues exist', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-full-error',
        started_at: '2026-03-12T09:56:00Z',
        completed_at: '2026-03-12T09:57:00Z',
        duration_ms: 60000,
        type: 'patrol',
        trigger_reason: 'startup',
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
        scope_resource_types: [],
        resources_checked: 58,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 1,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: '1 warning',
        finding_ids: ['finding-1'],
        error_count: 1,
        status: 'error',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);
    intelligenceState.findings = [
      {
        id: 'finding-1',
        source: 'ai-patrol',
        isThreshold: false,
        status: 'active',
        severity: 'warning',
        resourceId: 'ai-service',
        resourceName: 'Pulse Patrol Service',
        resourceType: 'service',
        title: 'Pulse Patrol: Insufficient API credits',
        detectedAt: '2026-03-12T09:57:00Z',
      },
    ];
    intelligenceState.summary = {
      timestamp: '2026-03-12T10:00:00Z',
      overall_health: {
        score: 60,
        grade: 'C',
        trend: 'stable',
        factors: [
          {
            name: 'Patrol coverage incomplete',
            impact: -0.35,
            description:
              'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
            category: 'coverage',
          },
        ],
        prediction:
          'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
      },
      findings_count: {
        critical: 0,
        warning: 1,
        watch: 0,
        info: 0,
        total: 1,
      },
      predictions_count: 0,
      recent_changes_count: 0,
      learning: {
        resources_with_knowledge: 0,
        total_notes: 0,
        resources_with_baselines: 0,
        patterns_detected: 0,
        correlations_learned: 0,
        incidents_tracked: 0,
      },
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByText('Patrol runtime issue')).toBeInTheDocument();
      expect(
        screen.getByText(
          'Patrol has an active runtime issue: Insufficient API credits. Recent coverage is also incomplete, so the rest of your infrastructure is not fully verified.',
        ),
      ).toBeInTheDocument();
    });

    expect(screen.getByText(/Assessment C · 60\/100/)).toBeInTheDocument();

    expect(screen.getByRole('button', { name: 'Findings' }).textContent).toBe('Findings 1');

    expect(
      screen.queryByText(
        'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
      ),
    ).not.toBeInTheDocument();
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

    fireEvent.click(screen.getByRole('button', { name: 'Runs' }));
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

    fireEvent.click(screen.getByRole('button', { name: 'Runs' }));
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

  it('does not turn a missing snapshot finding list into an empty snapshot filter', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    runHistoryState.selection = {
      id: 'run-missing-snapshot',
      started_at: '2026-03-12T10:10:00Z',
      completed_at: '2026-03-12T10:11:00Z',
      duration_ms: 60000,
      type: 'scoped',
      trigger_reason: 'alert_fired',
      scope_resource_ids: ['seed-resource'],
      effective_scope_resource_ids: ['expanded-a'],
      scope_resource_types: ['vm'],
      resources_checked: 1,
      nodes_checked: 0,
      guests_checked: 1,
      docker_checked: 0,
      storage_checked: 0,
      hosts_checked: 0,
      pbs_checked: 0,
      pmg_checked: 0,
      kubernetes_checked: 0,
      new_findings: 0,
      existing_findings: 1,
      rejected_findings: 0,
      resolved_findings: 0,
      auto_fix_count: 0,
      findings_summary: 'Legacy run without snapshot ids',
      error_count: 0,
      status: 'issues_found',
      tool_call_count: 0,
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(getPatrolStatusMock).toHaveBeenCalled();
      expect(findingsPanelState.latestProps).not.toBeNull();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Runs' }));
    fireEvent.click(screen.getByRole('button', { name: 'Select mocked run' }));
    fireEvent.click(screen.getByRole('button', { name: 'Findings' }));

    await waitFor(() => {
      expect(screen.getByText(/Filtered to run/i)).toBeInTheDocument();
    });

    expect(findingsPanelState.latestProps).toMatchObject({
      filterOverride: 'all',
      filterFindingIds: undefined,
      scopeResourceIds: ['expanded-a'],
      scopeResourceTypes: ['vm'],
      showScopeWarnings: true,
    });
  });
});
