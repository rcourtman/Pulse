import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Suspense, createSignal } from 'solid-js';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import { getPublicPricingUrl } from '@/utils/pricingHandoff';
import patrolIntelligenceStateSource from '@/features/patrol/usePatrolIntelligenceState.ts?raw';

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
      runSnapshotId?: string;
    } | null,
  },
  runHistoryState: {
    selection: null as Record<string, unknown> | null,
  },
  intelligenceState: {
    findings: [] as Array<Record<string, unknown>>,
    circuitBreakerStatus: null as { state: string; consecutive_failures: number } | null,
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
      recent_changes?: Array<{
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
const loadCommercialPostureMock = vi.fn();
const getUpgradeActionDestinationMock = vi.fn();
const getUpgradeActionUrlOrFallbackMock = vi.fn();
const presentationPolicyHidesUpgradePromptsMock = vi.fn();
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
    getModels: (...args: unknown[]) => apiFetchJSONMock('/api/ai/models', ...args),
    getSettings: (...args: unknown[]) => apiFetchJSONMock('/api/settings/ai', ...args),
    updateSettings: (...args: unknown[]) => apiFetchJSONMock('/api/settings/ai/update', ...args),
  },
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  loadRuntimeCapabilities: (...args: unknown[]) => loadLicenseStatusMock(...args),
}));

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: (...args: unknown[]) => getUpgradeActionDestinationMock(...args),
  commercialPosture: (...args: unknown[]) => licenseStatusMock(...args),
  licenseStatus: (...args: unknown[]) => licenseStatusMock(...args),
  loadCommercialPosture: (...args: unknown[]) => loadCommercialPostureMock(...args),
  loadRuntimeCapabilities: (...args: unknown[]) => loadLicenseStatusMock(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => getUpgradeActionUrlOrFallbackMock(...args),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => presentationPolicyHidesUpgradePromptsMock(),
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
    get patrolFindings() {
      return intelligenceState.findings;
    },
    get intelligenceSummary() {
      return intelligenceState.summary;
    },
    get circuitBreakerStatus() {
      return intelligenceState.circuitBreakerStatus;
    },
    get patrolPendingApprovals() {
      return [];
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
      runSnapshotId:
        props.runSnapshot &&
        typeof props.runSnapshot === 'object' &&
        typeof (props.runSnapshot as { id?: unknown }).id === 'string'
          ? ((props.runSnapshot as { id: string }).id ?? undefined)
          : undefined,
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
  PageHeader: (props: { title?: string; description?: string; actions?: unknown }) => (
    <div>
      <h1>{props.title}</h1>
      <p>{props.description}</p>
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
  PulsePatrolLogo: (props: { decorative?: boolean; title?: string }) => (
    <svg
      data-testid="pulse-patrol-logo"
      aria-hidden={props.decorative ? 'true' : undefined}
      aria-label={props.decorative ? undefined : (props.title ?? 'Pulse Patrol')}
    />
  ),
}));

const defaultPatrolStatus = (overrides: Record<string, unknown> = {}) => ({
  runtime_state: 'active',
  running: false,
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
  it('keeps Patrol page refresh affordance bounded when supporting reads stall', () => {
    expect(patrolIntelligenceStateSource).toContain('PATROL_REFRESH_TIMEOUT_MS');
    expect(patrolIntelligenceStateSource).toContain('finishRefresh(requestId)');
    expect(patrolIntelligenceStateSource).toContain('requestId === refreshRequestId');
  });

  beforeEach(() => {
    resetCreateNonSuspendingQueryCacheForTest();
    resetAIRuntimeState();
    getPatrolStatusMock.mockReset();
    getPatrolAutonomySettingsMock.mockReset();
    updatePatrolAutonomySettingsMock.mockReset();
    triggerPatrolRunMock.mockReset();
    getPatrolRunHistoryMock.mockReset();
    apiFetchJSONMock.mockReset();
    hasFeatureMock.mockReset();
    licenseStatusMock.mockReset();
    loadLicenseStatusMock.mockReset();
    loadCommercialPostureMock.mockReset();
    getUpgradeActionDestinationMock.mockReset();
    getUpgradeActionUrlOrFallbackMock.mockReset();
    presentationPolicyHidesUpgradePromptsMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    findingsPanelState.latestProps = null;
    runHistoryState.selection = null;
    intelligenceState.findings = [];
    intelligenceState.circuitBreakerStatus = null;
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
    loadCommercialPostureMock.mockResolvedValue(undefined);
    getUpgradeActionDestinationMock.mockImplementation((feature?: string) => ({
      href: getPublicPricingUrl(feature),
      external: true,
    }));
    getUpgradeActionUrlOrFallbackMock.mockImplementation((feature?: string) =>
      getPublicPricingUrl(feature),
    );
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(false);
    getCorrelationsMock.mockResolvedValue({
      correlations: [],
      count: 0,
    });
  });

  it('keeps the Patrol page heading accessible name singular when the logo is present', async () => {
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
    });

    expect(screen.queryByRole('heading', { name: 'Pulse Patrol Patrol' })).not.toBeInTheDocument();
    expect(screen.getByTestId('pulse-patrol-logo')).toHaveAttribute('aria-hidden', 'true');
  });

  it('surfaces Patrol readiness issues before a manual run can start', async () => {
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        readiness: {
          status: 'not_ready',
          ready: false,
          summary:
            'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
          provider: 'ollama',
          model: 'ollama:deepseek-r1:7b-llama-distill-q4_K_M',
          checks: [
            {
              id: 'tools',
              status: 'not_ready',
              label: 'Patrol tools',
              message:
                'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
              action: 'open_provider_settings',
            },
          ],
        },
      }),
    );

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByText('Patrol readiness issue')).toBeInTheDocument();
    });
    expect(
      screen.getByText(
        'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
      ),
    ).toBeInTheDocument();
    for (const button of screen.getAllByRole('button', { name: /Run Patrol/i })) {
      expect(button).toBeDisabled();
    }
    expect(triggerPatrolRunMock).not.toHaveBeenCalled();
  });

  it('surfaces Patrol readiness warnings without blocking manual runs', async () => {
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        readiness: {
          status: 'warning',
          ready: true,
          summary:
            'Ollama connectivity alone does not prove tool support. Use an Ollama model that returns tool_calls for Patrol verification.',
          provider: 'ollama',
          model: 'ollama:llama3',
          checks: [
            {
              id: 'tools',
              status: 'warning',
              label: 'Patrol tools',
              message:
                'Ollama connectivity alone does not prove tool support. Use an Ollama model that returns tool_calls for Patrol verification.',
              action: 'open_provider_settings',
            },
          ],
        },
      }),
    );

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByText('Patrol readiness warning')).toBeInTheDocument();
    });
    const runButtons = screen.getAllByRole('button', { name: /Run Patrol/i });
    for (const button of runButtons) {
      expect(button).not.toBeDisabled();
    }

    fireEvent.click(runButtons[0]);
    await waitFor(() => {
      expect(triggerPatrolRunMock).toHaveBeenCalled();
    });
  });

  it('surfaces backend readiness rejection when a stale manual run request reaches the server', async () => {
    triggerPatrolRunMock.mockRejectedValue(
      new Error(
        'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
      ),
    );

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByRole('button', { name: /Run Patrol/i })[0]);

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
      );
    });
  });

  it('keeps legacy Patrol status payloads without readiness compatible', async () => {
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ readiness: undefined }));

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
    });
    expect(screen.queryByText('Patrol readiness issue')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol readiness warning')).not.toBeInTheDocument();
    for (const button of screen.getAllByRole('button', { name: /Run Patrol/i })) {
      expect(button).not.toBeDisabled();
    }
  });

  it('renders canonical learned correlations in the summary page through the correlation card', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    intelligenceState.summary = {
      timestamp: '2026-03-01T00:00:00Z',
      overall_health: {
        score: 63,
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
      recent_changes: [],
      policy_posture: {
        total_resources: 3,
        sensitivity_counts: {},
        routing_counts: {},
      },
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
      expect(screen.getByText('Supporting context')).toBeInTheDocument();
    });

    expect(screen.queryByRole('heading', { name: 'Learned correlations' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'View supporting context' }));
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Learned correlations' })).toBeInTheDocument();
    });

    expect(screen.getByText('How to read this')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Findings and run history are Patrol verification evidence. The cards below add explanatory context and do not count as a fresh full patrol.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('2 learned patterns · explanatory context')).toBeInTheDocument();
    expect(screen.getByText('Coverage posture for policy-covered resources.')).toBeInTheDocument();
    expect(screen.getByText('3 policy-covered resources')).toBeInTheDocument();
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

  it('locks paid patrol controls without promoting checkout from ordinary Patrol workflows', async () => {
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
    expect(screen.getByRole('button', { name: 'Remediate' })).toBeDisabled();
    expect(
      screen.getByText(
        'Safe remediation workflows and alert-triggered root-cause analysis are not enabled on this plan.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Investigation and safe remediation workflows are not enabled on this plan.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Alert-triggered analysis is not enabled on this plan.'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade to Pro' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /start free trial/i })).not.toBeInTheDocument();
  });

  it('locks paid patrol controls without upgrade prompts in default self-hosted mode', async () => {
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(true);
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
    expect(screen.getByRole('button', { name: 'Remediate' })).toBeDisabled();
    expect(screen.queryByRole('link', { name: 'Upgrade to Pro' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /start free trial/i })).not.toBeInTheDocument();
  });

  it('keeps a saved direct-provider Patrol model selected after the catalog loads', async () => {
    apiFetchJSONMock.mockImplementation(async (path: string) => {
      if (path === '/api/ai/models') {
        return {
          models: [
            {
              id: 'deepseek:deepseek-v4-flash',
              name: 'DeepSeek V4 Flash',
              description: 'DeepSeek: current V4 Flash direct API model',
            },
          ],
        };
      }
      if (path === '/api/settings/ai') {
        return {
          ...defaultAISettings,
          model: 'openrouter:minimax/minimax-m2.5',
          patrol_model: 'deepseek:deepseek-v4-flash',
        };
      }
      if (path === '/api/settings/ai/update') {
        return defaultAISettings;
      }
      return {};
    });

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/ai/models');
    });

    fireEvent.click(screen.getByRole('button', { name: 'Configure Patrol' }));

    const select = screen.getByLabelText('Provider model') as HTMLSelectElement;
    await waitFor(() => {
      expect(select.value).toBe('deepseek:deepseek-v4-flash');
    });
    expect(select.selectedOptions[0]?.textContent).toContain('DeepSeek V4 Flash');
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
    expect(screen.getByRole('button', { name: 'Remediate' })).not.toBeDisabled();
    expect(screen.queryByRole('link', { name: 'Upgrade to Pro' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade' })).not.toBeInTheDocument();
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

    expect(screen.getByText(/Health A · 91\/100/)).toBeInTheDocument();
    expect(screen.queryByText('Supporting context')).not.toBeInTheDocument();
    expect(
      screen.queryByText('1 recent change · 4 policy-covered resources'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Policy posture')).not.toBeInTheDocument();
  });

  it('renders a stable patrol summary loading shell before the first assessment payload arrives', async () => {
    getPatrolStatusMock.mockImplementation(() => new Promise(() => {}));
    intelligenceState.summary = null;

    render(() => (
      <Suspense fallback={<div>Loading view...</div>}>
        <AIIntelligence />
      </Suspense>
    ));

    await waitFor(() => {
      expect(screen.getByTestId('patrol-summary-loading')).toBeInTheDocument();
    });

    expect(screen.queryByText('Loading view...')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol assessment')).not.toBeInTheDocument();
  });

  it('does not present a healthy patrol summary when a retired hosted block reason is normalized', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        runtime_state: 'blocked',
        blocked_reason:
          'Quickstart credits exhausted. Connect your API key to continue using Patrol.',
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
      recent_changes: [],
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
      screen.getAllByText('Connect your own AI provider or local model to use Pulse Patrol.')
        .length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText(/Quickstart credits exhausted/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Health A · 100\/100/)).not.toBeInTheDocument();
  });

  it('does not show retired hosted-credit chips when patrol is active on a configured provider path', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        runtime_state: 'active',
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

    expect(screen.queryByText('Patrol quickstart exhausted')).not.toBeInTheDocument();
    expect(screen.getByText(/Health A · 100\/100/)).toBeInTheDocument();
  });

  it('normalizes retired hosted availability copy on unactivated installs', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'expired' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        runtime_state: 'blocked',
        blocked_reason:
          'Connect your API key to use AI Patrol on this install. Hosted quickstart requires an activated entitlement.',
      }),
    );

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(getPatrolStatusMock).toHaveBeenCalled();
      expect(
        screen.getAllByText('Connect your own AI provider or local model to use Pulse Patrol.')
          .length,
      ).toBeGreaterThan(0);
    });

    expect(screen.queryByText('Patrol quickstart exhausted')).not.toBeInTheDocument();
    expect(screen.queryByText(/Hosted quickstart requires/i)).not.toBeInTheDocument();
  });

  it('does not surface hosted-credit chips even when legacy runtime fields are present', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        runtime_state: 'active',
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

    expect(screen.queryByText(/Patrol quickstart/i)).not.toBeInTheDocument();
    expect(screen.queryByTitle(/No API key needed/i)).not.toBeInTheDocument();
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
        truenas_checked: 0,
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
      expect(screen.getByText('Next:')).toBeInTheDocument();
      expect(screen.getByText('Verify full coverage')).toBeInTheDocument();
      expect(screen.getByTestId('patrol-recommended-next-step-action')).toHaveTextContent(
        'Run Patrol',
      );
      expect(findingsPanelState.latestProps).not.toBeNull();
    });

    // Patrol summary verification + activity-mix copy live behind the
    // "Show details" toggle since commit 174d2b04d. Expand the section so
    // the deeper assertions can find the text.
    fireEvent.click(screen.getByTestId('patrol-summary-details-toggle'));

    await waitFor(() => {
      expect(screen.getByText('No recent full patrol')).toBeInTheDocument();
      expect(screen.getAllByText(/Last activity/i)).toHaveLength(1);
    });

    expect(screen.queryByText('No issues found')).not.toBeInTheDocument();
    expect(screen.getByTestId('patrol-summary-details')).toHaveTextContent(
      'Run a full Patrol sweep before treating this assessment as an all-clear; recent evidence is incomplete or limited to targeted activity.',
    );
    expect(screen.queryByText(/Last patrol/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Last:/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Partial verification')).not.toBeInTheDocument();
    expect(screen.queryByText('Active findings')).not.toBeInTheDocument();
    expect(screen.queryByText('Warnings')).not.toBeInTheDocument();
    expect(screen.queryByText('Critical')).not.toBeInTheDocument();
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

  it('prefers last activity transport over last full patrol transport when no run history is loaded', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        runtime_state: 'active',
        last_patrol_at: '2026-03-12T09:30:00Z',
        last_activity_at: '2026-03-12T09:59:00Z',
      }),
    );
    getPatrolRunHistoryMock.mockResolvedValue([]);
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
      expect(screen.getByText(/Last activity:/i)).toBeInTheDocument();
    });

    expect(screen.queryByText(/Last full patrol:/i)).not.toBeInTheDocument();
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
        truenas_checked: 0,
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
        title: 'Pulse Patrol: Provider billing or quota issue',
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
    });

    // "Runtime issues" and "Latest activity" both live in the collapsible
    // Patrol summary detail section (commit 174d2b04d).
    fireEvent.click(screen.getByTestId('patrol-summary-details-toggle'));

    await waitFor(() => {
      expect(
        screen.getByText(
          'Patrol has an active runtime issue: Provider billing or quota issue. Recent coverage is also incomplete, so the rest of your infrastructure is not fully verified.',
        ),
      ).toBeInTheDocument();
      expect(screen.getByText('Runtime issues')).toBeInTheDocument();
      expect(screen.getByText('Latest activity')).toBeInTheDocument();
    });

    expect(screen.getByText(/Assessment C · 60\/100/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Patrol provider settings' })).toHaveAttribute(
      'href',
      '/settings/system-ai',
    );
    const assessmentShell = screen.getByText('Patrol assessment').closest('section');
    expect(assessmentShell).not.toBeNull();
    expect(assessmentShell!.className).toContain('border-y');
    expect(assessmentShell!.className).not.toContain('rounded-md');
    expect(assessmentShell!.className).not.toContain('shadow-sm');
    expect(assessmentShell!.className).not.toContain('bg-amber-50');
    expect(screen.queryByText('Infrastructure findings')).not.toBeInTheDocument();
    expect(screen.queryByText('Warnings')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-status-bar')).not.toBeInTheDocument();

    expect(screen.getByRole('button', { name: 'Findings' }).textContent).toBe('Findings 1');

    expect(
      screen.queryByText(
        'Patrol coverage is incomplete: recent activity was limited to scoped runs and ended with errors, so overall health is not fully verified.',
      ),
    ).not.toBeInTheDocument();
  });

  it('does not repeat stale coverage caveats after a successful full patrol verified resources', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-full-success',
        started_at: '2026-03-12T09:56:00Z',
        completed_at: '2026-03-12T09:57:00Z',
        duration_ms: 60000,
        type: 'patrol',
        trigger_reason: 'manual',
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
        scope_resource_types: [],
        resources_checked: 58,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
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
        error_count: 0,
        status: 'issues_found',
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
        resourceId: 'backup-1',
        resourceName: 'delly',
        resourceType: 'backup',
        title: 'Backup failed',
        detectedAt: '2026-03-12T09:57:00Z',
      },
    ];
    intelligenceState.summary = {
      timestamp: '2026-03-12T10:00:00Z',
      overall_health: {
        score: 85,
        grade: 'B',
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
          'Recent Patrol runs encountered errors, so the current health summary may be incomplete.',
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
      expect(screen.getByText(/verified 58 resources/i)).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId('patrol-summary-details-toggle'));
    expect(screen.getByTestId('patrol-summary-details')).toHaveTextContent(
      'Patrol surfaced 1 active warning finding in your infrastructure. Review the active findings for more detail.',
    );

    expect(
      screen.queryByText(
        'Recent Patrol runs encountered errors, so the current health summary may be incomplete.',
      ),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Recent coverage is also incomplete/i)).not.toBeInTheDocument();
  });

  it('surfaces the recent activity mix in the verification summary when scoped runs are creating noise', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    getPatrolRunHistoryMock.mockResolvedValue([
      {
        id: 'run-scoped-alert',
        started_at: '2026-03-12T10:00:00Z',
        completed_at: '2026-03-12T10:01:00Z',
        duration_ms: 60000,
        type: 'scoped',
        trigger_reason: 'alert_fired',
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
        scope_resource_types: [],
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
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
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
      {
        id: 'run-scoped-anomaly',
        started_at: '2026-03-12T09:58:00Z',
        completed_at: '2026-03-12T09:59:00Z',
        duration_ms: 60000,
        type: 'scoped',
        trigger_reason: 'anomaly',
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
        scope_resource_types: [],
        resources_checked: 1,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
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
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
      {
        id: 'run-full',
        started_at: '2026-03-12T09:50:00Z',
        completed_at: '2026-03-12T09:57:00Z',
        duration_ms: 420000,
        type: 'patrol',
        trigger_reason: 'scheduled',
        scope_resource_ids: [],
        effective_scope_resource_ids: [],
        scope_resource_types: [],
        resources_checked: 58,
        nodes_checked: 0,
        guests_checked: 0,
        docker_checked: 0,
        storage_checked: 0,
        hosts_checked: 0,
        truenas_checked: 0,
        pbs_checked: 0,
        pmg_checked: 0,
        kubernetes_checked: 0,
        new_findings: 0,
        existing_findings: 0,
        rejected_findings: 0,
        resolved_findings: 0,
        auto_fix_count: 0,
        findings_summary: 'No active findings',
        finding_ids: [],
        error_count: 0,
        status: 'healthy',
        triage_flags: 0,
        tool_call_count: 0,
      },
    ]);
    intelligenceState.summary = {
      timestamp: '2026-03-12T10:05:00Z',
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

    // "Recent activity mix" copy lives in the collapsible Patrol summary
    // detail section (commit 174d2b04d). Expand before asserting.
    await waitFor(() => {
      expect(screen.getByTestId('patrol-summary-details-toggle')).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId('patrol-summary-details-toggle'));

    await waitFor(() => {
      expect(
        screen.getByText('Recent activity mix: 1 full, 1 alert-triggered, 1 anomaly-triggered'),
      ).toBeInTheDocument();
    });
  });

  it('treats a selected zero-finding run as an empty snapshot and uses effective scope ids', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    intelligenceState.findings = [
      {
        id: 'finding-runtime',
        status: 'active',
        severity: 'warning',
        resourceId: 'ai-service',
        resourceName: 'Pulse Patrol Service',
        title: 'Pulse Patrol: Provider billing or quota issue',
      },
    ];
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
      truenas_checked: 0,
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

    expect(screen.queryByText('Findings snapshot unavailable')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Findings' }).textContent).toBe('Findings');

    expect(findingsPanelState.latestProps).toMatchObject({
      filterOverride: 'all',
      filterFindingIds: [],
      scopeResourceIds: ['expanded-a', 'expanded-b'],
      scopeResourceTypes: ['vm'],
      showScopeWarnings: true,
      runSnapshotId: 'run-empty',
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
      truenas_checked: 0,
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

    expect(screen.queryByText('Findings snapshot unavailable')).not.toBeInTheDocument();
    expect(findingsPanelState.latestProps).toMatchObject({
      filterOverride: 'all',
      filterFindingIds: [],
      scopeResourceIds: [],
      scopeResourceTypes: ['vm'],
      showScopeWarnings: true,
      runSnapshotId: 'run-empty-effective-scope',
    });
  });

  it('does not turn a missing snapshot finding list into an empty snapshot filter', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    intelligenceState.findings = [
      {
        id: 'finding-runtime',
        status: 'active',
        severity: 'warning',
        resourceId: 'ai-service',
        resourceName: 'Pulse Patrol Service',
        title: 'Pulse Patrol: Provider billing or quota issue',
      },
    ];
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
      truenas_checked: 0,
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

    expect(screen.getByText(/Findings snapshot unavailable/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Findings' }).textContent).toBe('Findings');

    expect(findingsPanelState.latestProps).toMatchObject({
      filterOverride: 'all',
      filterFindingIds: undefined,
      scopeResourceIds: ['expanded-a'],
      scopeResourceTypes: ['vm'],
      showScopeWarnings: true,
      runSnapshotId: 'run-missing-snapshot',
    });
  });
});
