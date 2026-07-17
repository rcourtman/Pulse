import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Suspense, createSignal, type JSX } from 'solid-js';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import {
  PATROL_CONTROL_ANCHOR,
  PATROL_CONTROL_STARTER,
  PATROL_CONTROL_STARTER_QUERY_PARAM,
  PATROL_OPERATIONS_LOOP_ANCHOR,
} from '@/routing/resourceLinks';
import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import {
  getPublicPricingUrl,
  SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
} from '@/utils/pricingHandoff';
import patrolIntelligenceHeaderSource from '@/features/patrol/PatrolIntelligenceHeader.tsx?raw';
import patrolIntelligenceStateSource from '@/features/patrol/usePatrolIntelligenceState.ts?raw';
import { AGENT_PATROL_CONTROL_STATUS_PATH } from '@/api/agentCapabilities';

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
const runtimeCapabilitiesMock = vi.fn();
const getRuntimeCapabilityBlockMock = vi.fn();
const licenseStatusMock = vi.fn();
const loadLicenseStatusMock = vi.fn();
const loadCommercialPostureMock = vi.fn();
const getUpgradeActionDestinationMock = vi.fn();
const getUpgradeActionUrlOrFallbackMock = vi.fn();
const presentationPolicyHidesUpgradePromptsMock = vi.fn();
const presentationPolicyHidesCommercialSurfacesMock = vi.fn();

vi.mock('@solidjs/router', () => ({
  A: (props: {
    href: string;
    class?: string;
    children?: JSX.Element;
    'aria-label'?: string;
    title?: string;
  }) => (
    <a href={props.href} class={props.class} aria-label={props['aria-label']} title={props.title}>
      {props.children}
    </a>
  ),
  useLocation: () => ({ hash: '', pathname: '/patrol', search: '' }),
}));
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const recordWorkflowPromptActivityMock = vi.fn();

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

vi.mock('@/api/aiChat', () => ({
  AIChatAPI: {
    recordWorkflowPromptActivity: (...args: unknown[]) => recordWorkflowPromptActivityMock(...args),
  },
  PULSE_OPERATIONS_LOOP_WORKFLOW_PROMPT_NAME: 'pulse_operations_loop',
  PULSE_PATROL_WORKFLOW_PROMPT_SURFACE: 'pulse_patrol',
  PULSE_PATROL_CONTROL_WORKFLOW_PROMPT_SURFACE: 'patrol_control',
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSONMock(...args),
}));

vi.mock('@/stores/license', () => ({
  hasFeature: (...args: unknown[]) => hasFeatureMock(...args),
  getRuntimeCapabilityBlock: (...args: unknown[]) => getRuntimeCapabilityBlockMock(...args),
  runtimeCapabilities: (...args: unknown[]) => runtimeCapabilitiesMock(...args),
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
  presentationPolicyHidesCommercialSurfaces: () => presentationPolicyHidesCommercialSurfacesMock(),
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
    loadPatrolFindings: vi.fn().mockResolvedValue(undefined),
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
    ariaLabelledBy?: string;
    ariaDescribedBy?: string;
  }) => (
    <button
      type="button"
      aria-label={props.ariaLabel}
      aria-labelledby={props.ariaLabel ? undefined : props.ariaLabelledBy}
      aria-describedby={props.ariaDescribedBy}
      aria-pressed={props.checked}
      disabled={props.disabled}
      onClick={() => props.onToggle?.(!props.checked)}
    />
  ),
  Toggle: (props: {
    checked?: boolean;
    disabled?: boolean;
    onChange?: (event: Event & { currentTarget: HTMLInputElement }) => void;
    ariaLabel?: string;
    ariaLabelledBy?: string;
    ariaDescribedBy?: string;
  }) => (
    <input
      type="checkbox"
      checked={props.checked}
      disabled={props.disabled}
      aria-label={props.ariaLabel}
      aria-labelledby={props.ariaLabel ? undefined : props.ariaLabelledBy}
      aria-describedby={props.ariaDescribedBy}
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

const defaultAgentCapabilitiesManifest = () => ({
  version: 'v1',
  surfaceContract: {
    core: {
      id: 'pulse_intelligence_core',
      label: 'Pulse Intelligence Core',
      description:
        'Canonical context, governed actions, safety gates, approval state, action audit, and verification.',
    },
    proactiveEngine: {
      id: 'pulse_patrol',
      label: 'Pulse Patrol',
      description: 'The proactive detection and investigation engine.',
    },
    operatorSurfaces: [
      {
        id: 'pulse_mcp',
        label: 'Pulse MCP',
        description: 'The external-agent adapter.',
        native: false,
        externalAdapter: true,
        affordances: {
          tools: true,
          resources: true,
          prompts: true,
          capabilityMetadata: true,
        },
      },
    ],
  },
  surfaceToolContracts: [
    {
      surfaceId: 'pulse_mcp',
      surfaceLabel: 'Pulse MCP',
      toolSource: 'capability_manifest',
      toolNames: [
        'get_patrol_control_status',
        'get_fleet_context',
        'get_resource_context',
        'list_findings',
        'plan_action',
        'decide_action',
        'execute_action',
        'resolve_finding',
      ],
      capabilityNames: [
        'get_patrol_control_status',
        'get_fleet_context',
        'get_resource_context',
        'list_findings',
        'plan_action',
        'decide_action',
        'execute_action',
        'resolve_finding',
      ],
      affordances: {
        tools: true,
        resources: true,
        prompts: true,
        capabilityMetadata: true,
      },
    },
  ],
  mcpAdapter: {
    serverName: 'pulse',
    command: 'pulse-mcp',
    baseUrlFlag: '--base-url',
    defaultBaseUrl: 'http://localhost:7655',
    tokenEnv: 'PULSE_API_TOKEN',
    configFamilies: [{ id: 'opencode', label: 'OpenCode', shape: 'opencode_mcp' }],
  },
  requiredScopes: [],
  categories: [],
  workflowPrompts: [{ name: 'pulse_operations_loop', label: 'Ask Patrol to handle an issue' }],
  capabilities: [],
});

const defaultPatrolAutonomySettings = (overrides: Record<string, unknown> = {}) => ({
  autonomy_level: 'monitor',
  requested_autonomy_level: 'monitor',
  effective_autonomy_level: 'monitor',
  full_mode_unlocked: false,
  autopilot_acknowledgement: {
    code: 'not_requested',
    active: false,
    currentVersion: 1,
    acceptedScope: [],
    acceptedLimits: {
      policyAllowlistRequired: true,
      emergencyStopHonored: true,
      approvalFloorsHonored: true,
      verificationReconciledWhenSupported: true,
      evidenceClassDisclosed: true,
      inconclusiveOutcomeAllowed: true,
      executionSuccessIsNotOutcomeTruth: true,
    },
  },
  investigation_budget: 15,
  investigation_timeout_sec: 300,
  ...overrides,
});

const defaultOperationsLoopStatus = (overrides: Record<string, unknown> = {}) => ({
  nextAction: 'run_patrol',
  progressLabel: 'Run Patrol to produce actionable issue evidence.',
  steps: [
    { id: 'patrol', label: 'Patrol', status: 'current', count: 0 },
    { id: 'assistant', label: 'Assistant', status: 'pending' },
    { id: 'governance', label: 'Governance', status: 'pending', count: 0 },
    { id: 'verification', label: 'Verification', status: 'pending', count: 0 },
  ],
  patrolEvidenceCount: 0,
  patrolIssueEvidenceCount: 0,
  activeFindingCount: 0,
  pendingApprovalCount: 0,
  governedActionCount: 0,
  approvedDecisionCount: 0,
  rejectedDecisionCount: 0,
  verifiedOutcomeCount: 0,
  operationsLoopStarterCount: 0,
  assistantOperationsLoopStarterCount: 0,
  patrolOperationsLoopStarterCount: 0,
  patrolControlOperationsLoopStarterCount: 0,
  patrolControlCompletedOperationsLoopCount: 0,
  patrolControlResolvedOperationsLoopCount: 0,
  patrolControlValueState: 'not_started',
  patrolAutonomyOperationsLoopStarterCount: 0,
  patrolAutonomyCompletedOperationsLoopCount: 0,
  patrolAutonomyResolvedOperationsLoopCount: 0,
  patrolAutonomyValueState: 'not_started',
  proActivationOperationsLoopStarterCount: 0,
  proActivationCompletedOperationsLoopCount: 0,
  proActivationResolvedOperationsLoopCount: 0,
  proActivationValueProofState: 'not_started',
  mcpOperationsLoopStarterCount: 0,
  externalAgentReady: true,
  windowStart: '2026-06-01T00:00:00Z',
  generatedAt: '2026-06-20T00:00:00Z',
  ...overrides,
});

describe('AIIntelligence entitlement gating', () => {
  it('keeps Patrol page data sync bounded without making it a primary action', () => {
    expect(patrolIntelligenceStateSource).toContain('PATROL_REFRESH_TIMEOUT_MS');
    expect(patrolIntelligenceStateSource).toContain('finishRefresh(requestId)');
    expect(patrolIntelligenceStateSource).toContain('requestId === refreshRequestId');
    expect(patrolIntelligenceStateSource).toContain('isManualRefreshRunning');
    expect(patrolIntelligenceStateSource).toContain('handleRefreshPatrol');
    expect(patrolIntelligenceStateSource).toContain('loadVisiblePatrolData');
    expect(patrolIntelligenceStateSource).toContain('loadSupportingPatrolDataInBackground');
    expect(patrolIntelligenceHeaderSource).not.toContain('state.handleRefreshPatrol()');
    expect(patrolIntelligenceHeaderSource).not.toContain('Sync page data');
    expect(patrolIntelligenceHeaderSource).not.toContain('animate-spin');
    expect(patrolIntelligenceHeaderSource).not.toContain('Refresh Patrol');
  });

  it('keeps the advanced Patrol settings drawer out of the old save-spinner path', () => {
    expect(patrolIntelligenceHeaderSource).not.toContain('LoadingSpinner');
    expect(patrolIntelligenceHeaderSource).not.toContain('Save Patrol mode');
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
    runtimeCapabilitiesMock.mockReset();
    getRuntimeCapabilityBlockMock.mockReset();
    licenseStatusMock.mockReset();
    loadLicenseStatusMock.mockReset();
    loadCommercialPostureMock.mockReset();
    getUpgradeActionDestinationMock.mockReset();
    getUpgradeActionUrlOrFallbackMock.mockReset();
    presentationPolicyHidesUpgradePromptsMock.mockReset();
    presentationPolicyHidesCommercialSurfacesMock.mockReset();
    notificationSuccessMock.mockReset();
    notificationErrorMock.mockReset();
    recordWorkflowPromptActivityMock.mockReset();
    findingsPanelState.latestProps = null;
    runHistoryState.selection = null;
    intelligenceState.findings = [];
    intelligenceState.circuitBreakerStatus = null;
    intelligenceState.summary = null;
    intelligenceState.correlations = null;
    setCorrelationsState(null);
    getCorrelationsMock.mockReset();

    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus());
    getPatrolAutonomySettingsMock.mockResolvedValue(defaultPatrolAutonomySettings());
    updatePatrolAutonomySettingsMock.mockResolvedValue({
      settings: defaultPatrolAutonomySettings(),
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
      if (path === '/api/agent/capabilities') {
        return defaultAgentCapabilitiesManifest();
      }
      if (path === AGENT_PATROL_CONTROL_STATUS_PATH) {
        return defaultOperationsLoopStatus();
      }
      return {};
    });
    hasFeatureMock.mockImplementation(
      (feature: string) => !['ai_alerts', 'ai_autofix'].includes(feature),
    );
    runtimeCapabilitiesMock.mockReturnValue({
      runtime: 'community',
      capabilities: ['ai_intelligence'],
      blocked_capabilities: [],
    });
    getRuntimeCapabilityBlockMock.mockReturnValue(undefined);
    licenseStatusMock.mockReturnValue({ subscription_state: 'expired' });
    loadLicenseStatusMock.mockResolvedValue(undefined);
    loadCommercialPostureMock.mockResolvedValue(undefined);
    getUpgradeActionDestinationMock.mockImplementation((feature?: string) =>
      feature === 'ai_autofix'
        ? { href: SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF, external: false }
        : {
            href: getPublicPricingUrl(feature),
            external: true,
          },
    );
    getUpgradeActionUrlOrFallbackMock.mockImplementation((feature?: string) =>
      getPublicPricingUrl(feature),
    );
    presentationPolicyHidesUpgradePromptsMock.mockReturnValue(false);
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(false);
    recordWorkflowPromptActivityMock.mockResolvedValue(undefined);
    window.history.replaceState({}, '', '/patrol');
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

  it('keeps the Patrol route handoff anchored without rendering idle run prompts as current work', async () => {
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
    });

    expect(
      screen.getByText('Patrol checks your infrastructure and shows current issues.'),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        'Patrol checks your infrastructure, explains what it found, follows your mode before acting, and records the result.',
      ),
    ).not.toBeInTheDocument();
    const patrolControlAnchor = document.getElementById(PATROL_CONTROL_ANCHOR);
    const operationsLoopAnchor = document.getElementById(PATROL_OPERATIONS_LOOP_ANCHOR);
    expect(patrolControlAnchor).not.toBeNull();
    const patrolControl = within(patrolControlAnchor!);
    expect(patrolControl.queryByRole('group', { name: 'Patrol mode' })).not.toBeInTheDocument();
    expect(patrolControl.getByText('Patrol mode')).toBeInTheDocument();
    expect(patrolControl.getAllByText('Watch only').length).toBeGreaterThan(0);
    expect(
      patrolControl.getByText(
        'Patrol checks infrastructure and reports issues only; it does not start fixes.',
      ),
    ).toBeInTheDocument();
    expect(patrolControl.queryByRole('button', { name: 'Limits' })).not.toBeInTheDocument();
    expect(patrolControl.queryByRole('button', { name: 'Ask first' })).toBeNull();
    expect(patrolControl.queryByRole('button', { name: 'Safe auto-fix' })).toBeNull();
    expect(patrolControl.queryByRole('button', { name: 'Autopilot' })).toBeNull();
    expect(patrolControl.queryByText('Patrol handles')).not.toBeInTheDocument();
    expect(patrolControl.queryByText('May do')).not.toBeInTheDocument();
    expect(patrolControl.queryByText('Needs Pro for')).not.toBeInTheDocument();
    expect(patrolControl.queryByText('Will not')).not.toBeInTheDocument();
    expect(patrolControl.queryByText('Pro')).not.toBeInTheDocument();
    expect(patrolControl.queryByRole('button', { name: 'Ask first Pro' })).toBeNull();
    expect(patrolControl.queryByRole('button', { name: 'Safe auto-fix Pro' })).toBeNull();
    expect(patrolControl.queryByRole('button', { name: 'Autopilot Pro' })).toBeNull();
    expect(screen.getByRole('link', { name: 'Plans & Billing' })).toHaveAttribute(
      'href',
      SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
    );
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
    expect(operationsLoopAnchor?.parentElement).toBe(patrolControlAnchor);
    expect(screen.queryByTestId('patrol-current-work')).not.toBeInTheDocument();
    expect(screen.getByText('Current Patrol issues appear here.')).toBeInTheDocument();
    expect(
      screen.queryByText('Issues Patrol found. Infrastructure stays unchanged.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Current Patrol work')).not.toBeInTheDocument();
    expect(screen.queryByText('Ready to run')).not.toBeInTheDocument();
    expect(screen.queryByText('Current issue')).not.toBeInTheDocument();
    expect(screen.queryByText('Recorded outcome')).not.toBeInTheDocument();
    expect(screen.queryByText('Duty trail')).not.toBeInTheDocument();
    expect(screen.queryByText('External agents')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Pulse MCP' })).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Run Patrol' }).length).toBeGreaterThan(0);
  });

  it('consumes Patrol control route handoffs without loading legacy loop status', async () => {
    window.history.replaceState(
      {},
      '',
      `/patrol?${PATROL_CONTROL_STARTER_QUERY_PARAM}=${PATROL_CONTROL_STARTER}#${PATROL_CONTROL_ANCHOR}`,
    );

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(recordWorkflowPromptActivityMock).toHaveBeenCalledWith({
        name: 'pulse_operations_loop',
        surface: 'patrol_control',
      });
    });

    expect(
      apiFetchJSONMock.mock.calls.some(([path]) => path === AGENT_PATROL_CONTROL_STATUS_PATH),
    ).toBe(false);
    await waitFor(() => {
      expect(window.location.pathname).toBe('/patrol');
      expect(window.location.search).toBe('');
      expect(window.location.hash).toBe(`#${PATROL_CONTROL_ANCHOR}`);
    });
  });

  it('surfaces provider issues before a manual run can start', async () => {
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        trigger_status: {
          running: false,
          pending_triggers: 0,
          current_interval_ms: 300000,
          recent_events: 0,
          is_busy_mode: false,
          alert_triggers_enabled: true,
          anomaly_triggers_enabled: true,
          event_triggers_blocked: true,
          event_triggers_blocked_reason: 'background_automation_disabled',
          event_triggers_blocked_message:
            'Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.',
        },
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
      expect(screen.getByRole('heading', { name: 'Patrol needs setup' })).toBeInTheDocument();
    });
    expect(
      screen.getByText(
        'Patrol cannot check infrastructure until its selected model passes the Patrol tool check.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Selected model cannot run Patrol tools.')).toBeInTheDocument();
    expect(
      screen.queryByText(
        'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
      ),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Open work' })).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol setup issue')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol readiness issue')).not.toBeInTheDocument();
    expect(screen.queryByText(/Automation:/)).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        'Trigger status: Automatic Patrol checks from alerts and anomalies are paused by the local development safety guard. Manual Patrol still works.',
      ),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Trigger status:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/local development safety guard/)).not.toBeInTheDocument();
    const providerSettingsLinks = screen.getAllByRole('link', {
      name: 'Check Patrol model',
    });
    expect(providerSettingsLinks.length).toBeGreaterThan(0);
    expect(providerSettingsLinks[0]).toHaveAttribute('href', '/settings/pulse-intelligence/patrol');
    expect(screen.queryByRole('button', { name: /Run Patrol/i })).not.toBeInTheDocument();
    expect(triggerPatrolRunMock).not.toHaveBeenCalled();
  });

  it('surfaces provider warnings without blocking manual runs', async () => {
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        readiness: {
          status: 'warning',
          ready: true,
          summary:
            "Ollama connectivity alone does not prove tool support. qwen3:8b passes Patrol's tool check; run ollama pull qwen3:8b and select it as the Patrol model.",
          provider: 'ollama',
          model: 'ollama:llama3',
          checks: [
            {
              id: 'tools',
              status: 'warning',
              label: 'Patrol tools',
              message:
                "Ollama connectivity alone does not prove tool support. qwen3:8b passes Patrol's tool check; run ollama pull qwen3:8b and select it as the Patrol model.",
              action: 'open_provider_settings',
            },
          ],
        },
      }),
    );

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByText('Patrol model warning')).toBeInTheDocument();
    });
    expect(screen.queryByText('Patrol readiness warning')).not.toBeInTheDocument();
    const runButtons = screen.getAllByRole('button', { name: /Run Patrol/i });
    for (const button of runButtons) {
      expect(button).not.toBeDisabled();
    }

    fireEvent.click(runButtons[0]);
    await waitFor(() => {
      expect(triggerPatrolRunMock).toHaveBeenCalled();
    });
  });

  it('moves manual Patrol runs out of Starting state once the start request is accepted', async () => {
    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Open work' })).toBeInTheDocument();
    });

    getCorrelationsMock.mockImplementation(() => new Promise(() => {}));

    fireEvent.click(screen.getAllByRole('button', { name: /Run Patrol/i })[0]);

    await waitFor(() => {
      expect(triggerPatrolRunMock).toHaveBeenCalled();
      expect(screen.getAllByRole('button', { name: /Running/i }).length).toBeGreaterThan(0);
    });
    expect(screen.queryByRole('button', { name: /Starting/i })).not.toBeInTheDocument();
  });

  it('surfaces backend readiness rejection when a stale manual run request reaches the server', async () => {
    triggerPatrolRunMock.mockRejectedValue(
      new Error(
        'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
      ),
    );

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Open work' })).toBeInTheDocument();
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
    expect(screen.queryByText('Patrol setup issue')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol setup warning')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol readiness issue')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol readiness warning')).not.toBeInTheDocument();
    for (const button of screen.getAllByRole('button', { name: /Run Patrol/i })) {
      expect(button).not.toBeDisabled();
    }
  });

  it('does not expose raw supporting context as a Patrol page details panel', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    intelligenceState.findings = [
      {
        id: 'finding-storage-context',
        status: 'active',
        severity: 'warning',
        resourceId: 'storage-2',
        resourceName: 'Storage 2',
        title: 'Storage issue needs Patrol context',
      },
    ];
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
              'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
            category: 'coverage',
          },
        ],
        prediction:
          'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
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
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: 'Details' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Hide details' })).not.toBeInTheDocument();
    expect(
      screen.queryByText('Extra activity and policy context for the selected item.'),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText('Details available: related patterns, policy limits.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('For reference')).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        'This explains what Patrol saw. It does not change the finding or start a new patrol.',
      ),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Nearby activity' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Related patterns' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Policy limits' })).not.toBeInTheDocument();
    expect(screen.queryByText('2 related patterns Patrol can use')).not.toBeInTheDocument();
    expect(
      screen.queryByText('What Patrol was allowed to inspect or act on.'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Storage 2')).not.toBeInTheDocument();
    expect(screen.queryByText('VM 200')).not.toBeInTheDocument();
    expect(screen.queryByText('Disk Full → Restart')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'History' }));
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Patrol history' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: 'Details' })).not.toBeInTheDocument();
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

    const patrolControlAnchor = document.getElementById(PATROL_CONTROL_ANCHOR);
    expect(patrolControlAnchor).not.toBeNull();
    const patrolControl = within(patrolControlAnchor!);
    expect(patrolControl.queryByRole('group', { name: 'Patrol mode' })).not.toBeInTheDocument();
    expect(patrolControl.getByText('Patrol mode')).toBeInTheDocument();
    expect(patrolControl.getAllByText('Watch only').length).toBeGreaterThan(0);
    expect(patrolControl.queryByRole('button', { name: 'Limits' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Ask first, requires Pulse Pro' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Safe auto-fix, requires Pulse Pro' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Autopilot, requires Pulse Pro' })).toBeNull();
    expect(patrolControl.queryByText('Pro')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Ask first Pro' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Safe auto-fix Pro' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Autopilot Pro' })).toBeNull();
    expect(screen.getByRole('link', { name: 'Plans & Billing' })).toHaveAttribute(
      'href',
      SELF_HOSTED_PRO_BILLING_PLAN_SELECTION_HREF,
    );
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
    expect(screen.queryByText('Unlock Patrol mode')).not.toBeInTheDocument();
    expect(screen.queryByText('More Patrol modes')).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        'This plan can only watch. Pulse Pro unlocks investigation, approval-backed fixes, and automatic policy-approved fixes.',
      ),
    ).not.toBeInTheDocument();

    expect(screen.getByRole('link', { name: 'Open Patrol settings' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/patrol',
    );
    expect(screen.queryByRole('dialog', { name: 'Patrol schedule & model' })).toBeNull();
    expect(screen.queryByText('Container update risk')).not.toBeInTheDocument();
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

    const patrolControlAnchor = document.getElementById(PATROL_CONTROL_ANCHOR);
    expect(patrolControlAnchor).not.toBeNull();
    const patrolControl = within(patrolControlAnchor!);
    expect(patrolControl.queryByRole('group', { name: 'Patrol mode' })).not.toBeInTheDocument();
    expect(patrolControl.getByText('Patrol mode')).toBeInTheDocument();
    expect(patrolControl.getAllByText('Watch only').length).toBeGreaterThan(0);
    expect(patrolControl.queryByRole('button', { name: 'Limits' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Ask first, requires Pulse Pro' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Safe auto-fix, requires Pulse Pro' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Autopilot, requires Pulse Pro' })).toBeNull();
    expect(patrolControl.queryByText('Pro')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Ask first Pro' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Safe auto-fix Pro' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Autopilot Pro' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Plans & Billing' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Patrol settings' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/patrol',
    );
    expect(screen.queryByRole('dialog', { name: 'Patrol schedule & model' })).toBeNull();
    expect(screen.queryByText('This install can only watch')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol is in watch-only mode')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade to Pro' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /start free trial/i })).not.toBeInTheDocument();
  });

  it('hides paid patrol badges when commercial surfaces are hidden', async () => {
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(true);
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

    const patrolControlAnchor = document.getElementById(PATROL_CONTROL_ANCHOR);
    expect(patrolControlAnchor).not.toBeNull();
    const patrolControl = within(patrolControlAnchor!);
    expect(patrolControl.getAllByText('Watch only').length).toBeGreaterThan(0);
    expect(patrolControl.queryByText('Pro')).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'View plans' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade to Pro' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /start free trial/i })).not.toBeInTheDocument();
  });

  it('keeps the Patrol model catalog out of the operator page', async () => {
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

    expect(screen.getByRole('link', { name: 'Open Patrol settings' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/patrol',
    );
    expect(screen.queryByRole('dialog', { name: 'Patrol schedule & model' })).toBeNull();
    expect(screen.queryByText('DeepSeek V4 Flash')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Change model' })).toBeNull();
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

    expect(screen.getByRole('button', { name: 'Ask first' })).not.toBeDisabled();
    expect(screen.getByRole('button', { name: 'Safe auto-fix' })).not.toBeDisabled();
    expect(screen.getByRole('button', { name: 'Autopilot' })).not.toBeDisabled();
    expect(
      screen.getByText(
        'Patrol checks infrastructure and reports issues only; it does not start fixes.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Patrol handles')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Limits' })).not.toBeInTheDocument();
    expect(screen.queryByText('Hard limits')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Autopilot' }));

    // Autopilot no longer switches directly; it opens the acknowledgement
    // dialog and the mode stays where it was until the user records one.
    expect(await screen.findByRole('dialog', { name: 'Activate Autopilot' })).toBeInTheDocument();
    expect(updatePatrolAutonomySettingsMock).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Close Autopilot acknowledgement' }));

    expect(screen.getByRole('link', { name: 'Open Patrol settings' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/patrol',
    );
    expect(screen.queryByRole('dialog', { name: 'Patrol schedule & model' })).toBeNull();
    expect(screen.queryByRole('checkbox', { name: 'Container Update Risk' })).toBeNull();
    expect(screen.queryByRole('checkbox', { name: 'Alert-Triggered Patrols' })).toBeNull();
    expect(screen.queryByRole('checkbox', { name: 'Anomaly-Triggered Patrols' })).toBeNull();
    expect(screen.queryByRole('link', { name: 'Upgrade to Pro' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Upgrade' })).not.toBeInTheDocument();
  });

  it('records direct Patrol mode changes after successful paid control saves', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
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
      if (path === '/api/agent/capabilities') {
        return defaultAgentCapabilitiesManifest();
      }
      if (path === AGENT_PATROL_CONTROL_STATUS_PATH) {
        return defaultOperationsLoopStatus();
      }
      return {};
    });
    updatePatrolAutonomySettingsMock.mockResolvedValue({
      settings: defaultPatrolAutonomySettings({
        autonomy_level: 'approval',
        requested_autonomy_level: 'approval',
        effective_autonomy_level: 'approval',
      }),
    });

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Ask first' })).not.toBeDisabled();
    });
    const statusCallsBeforeModeChange = getPatrolStatusMock.mock.calls.length;
    const runHistoryCallsBeforeModeChange = getPatrolRunHistoryMock.mock.calls.length;

    fireEvent.click(screen.getByRole('button', { name: 'Ask first' }));

    await waitFor(() => {
      expect(updatePatrolAutonomySettingsMock).toHaveBeenCalledWith(
        expect.objectContaining({
          autonomy_level: 'approval',
          investigation_budget: 15,
          investigation_timeout_sec: 300,
        }),
      );
    });
    await waitFor(() => {
      expect(recordWorkflowPromptActivityMock).toHaveBeenCalledWith({
        name: 'pulse_operations_loop',
        surface: 'patrol_control',
      });
    });
    await waitFor(() => {
      expect(getPatrolStatusMock.mock.calls.length).toBeGreaterThan(statusCallsBeforeModeChange);
      expect(getPatrolRunHistoryMock.mock.calls.length).toBeGreaterThan(
        runHistoryCallsBeforeModeChange,
      );
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
      expect(screen.queryByTestId('patrol-current-work')).not.toBeInTheDocument();
      expect(screen.queryByText('Patrol ready')).not.toBeInTheDocument();
      expect(screen.queryByText(/Run Patrol now to look for issues/)).not.toBeInTheDocument();
      expect(screen.queryByText('Patrol mode ready')).not.toBeInTheDocument();
      expect(screen.queryByText(/Patrol mode is set/)).not.toBeInTheDocument();
    });
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
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
    });

    expect(screen.queryByText('Patrol status')).not.toBeInTheDocument();
    expect(screen.queryByText(/No active issues · health score 91\/100/)).not.toBeInTheDocument();
    expect(screen.queryByText('No active issues detected')).not.toBeInTheDocument();
    expect(screen.queryByText(/Health A · 91\/100/)).not.toBeInTheDocument();
    expect(screen.queryByText('Supporting context')).not.toBeInTheDocument();
    expect(
      screen.queryByText('1 recent change · 4 resources covered by policy'),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Policy posture')).not.toBeInTheDocument();
  });

  it('keeps the Patrol page visible without a separate summary loading strip', async () => {
    getPatrolStatusMock.mockImplementation(() => new Promise(() => {}));
    intelligenceState.summary = null;

    render(() => (
      <Suspense fallback={<div>Loading view...</div>}>
        <AIIntelligence />
      </Suspense>
    ));

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
      expect(screen.getByText('Open work')).toBeInTheDocument();
    });

    expect(screen.queryByText('Loading view...')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-loading')).not.toBeInTheDocument();
    expect(screen.queryByText('Loading Patrol')).not.toBeInTheDocument();
  });

  it('keeps the Patrol page visible when first-load refreshes fail', async () => {
    loadLicenseStatusMock.mockRejectedValue(new TypeError('Failed to fetch'));

    render(() => (
      <Suspense fallback={<div>Loading view...</div>}>
        <AIIntelligence />
      </Suspense>
    ));

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
      expect(screen.getByText('Patrol could not refresh')).toBeInTheDocument();
      expect(screen.getByText('Open work')).toBeInTheDocument();
    });

    expect(screen.queryByText('Loading view...')).not.toBeInTheDocument();
    expect(screen.queryByText('Failed to fetch')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
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
      expect(screen.getAllByText('Patrol paused').length).toBeGreaterThan(0);
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
    expect(screen.queryByText(/No active issues · health score 100\/100/)).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol status')).not.toBeInTheDocument();
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
              'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
            category: 'coverage',
          },
        ],
        prediction:
          'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
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
      expect(screen.getByText('Open work')).toBeInTheDocument();
      expect(screen.queryByText('Next:')).not.toBeInTheDocument();
      expect(screen.queryByText('Verify full coverage')).not.toBeInTheDocument();
      expect(screen.queryByTestId('patrol-recommended-next-step-action')).not.toBeInTheDocument();
      expect(findingsPanelState.latestProps).not.toBeNull();
    });

    expect(screen.queryByText('Coverage incomplete')).not.toBeInTheDocument();
    expect(screen.queryByText(/health score 70\/100/)).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details-toggle')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details')).not.toBeInTheDocument();
    expect(screen.queryByText('No recent full patrol')).not.toBeInTheDocument();
    expect(screen.queryByText('No active issues')).not.toBeInTheDocument();
    expect(screen.queryByText(/Last patrol/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Last:/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Partial verification')).not.toBeInTheDocument();
    expect(screen.queryByText('Active findings')).not.toBeInTheDocument();
    expect(screen.queryByText('Warnings')).not.toBeInTheDocument();
    expect(screen.queryByText('Critical')).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
      ),
    ).not.toBeInTheDocument();
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
              'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
            category: 'coverage',
          },
        ],
        prediction:
          'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
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
      expect(screen.getByRole('heading', { name: 'Patrol needs setup' })).toBeInTheDocument();
    });

    expect(
      screen.queryByText(/1 Patrol runtime issue · health score 60\/100/),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        'Patrol cannot check infrastructure until its selected model passes the Patrol tool check.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Patrol cannot run yet' })).toBeInTheDocument();
    expect(
      screen.queryByText('Fix the provider connection, then Patrol can check infrastructure.'),
    ).not.toBeInTheDocument();
    expect(screen.getByText('Provider billing or quota issue')).toBeInTheDocument();
    expect(screen.queryByText('Patrol setup issue')).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol readiness issue')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Check Patrol model' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/patrol',
    );
    expect(screen.queryByText('Runtime issue')).not.toBeInTheDocument();
    expect(screen.queryByText(/regressed \d+×/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Runtime issue/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Run Patrol' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Open Patrol settings' })).not.toBeInTheDocument();
    expect(screen.queryByText(/Automation:/)).not.toBeInTheDocument();
    const patrolControlAnchor = document.getElementById(PATROL_CONTROL_ANCHOR);
    expect(patrolControlAnchor).not.toBeNull();
    expect(patrolControlAnchor).toContainElement(
      screen.getByRole('group', { name: 'Patrol mode' }),
    );
    const patrolControl = within(patrolControlAnchor!);
    expect(patrolControl.getByText('Patrol mode')).toBeInTheDocument();
    expect(patrolControl.getAllByText('Watch only').length).toBeGreaterThan(0);
    expect(
      patrolControl.getByText(
        'Patrol checks infrastructure and reports issues only; it does not start fixes.',
      ),
    ).toBeInTheDocument();
    expect(patrolControl.getByRole('button', { name: 'Ask first' })).not.toBeDisabled();
    expect(patrolControl.getByRole('button', { name: 'Safe auto-fix' })).not.toBeDisabled();
    expect(patrolControl.getByRole('button', { name: 'Autopilot' })).not.toBeDisabled();
    expect(patrolControl.queryByRole('button', { name: 'Limits' })).not.toBeInTheDocument();
    expect(screen.queryByText(/Current mode:/)).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol status')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details-toggle')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details')).not.toBeInTheDocument();
    expect(screen.queryByText('Infrastructure findings')).not.toBeInTheDocument();
    expect(screen.queryByText('Warnings')).not.toBeInTheDocument();
    expect(screen.queryByText('Latest activity')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-status-bar')).not.toBeInTheDocument();

    expect(screen.queryByRole('heading', { name: 'Open work' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Active' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'All' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Resolved' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Details' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'History' })).not.toBeInTheDocument();

    expect(
      screen.queryByText(
        'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
      ),
    ).not.toBeInTheDocument();
  });

  it('keeps provider-blocked open issues to one provider action', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        license_required: false,
        readiness: {
          status: 'not_ready',
          ready: false,
          summary: 'Provider billing or quota issue.',
          provider: 'openrouter',
          model: 'openrouter:z-ai/glm-5.2',
        },
      }),
    );
    intelligenceState.findings = [
      {
        id: 'finding-storage-risk',
        source: 'ai-patrol',
        isThreshold: false,
        status: 'active',
        severity: 'critical',
        resourceId: 'storage:tower-array',
        resourceName: 'Tower Array',
        resourceType: 'storage',
        title: 'Unraid array running without parity protection',
        detectedAt: '2026-03-12T09:57:00Z',
      },
    ];

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(screen.getByText('Patrol model issue')).toBeInTheDocument();
    });
    expect(screen.getByRole('heading', { name: 'Open work' })).toBeInTheDocument();
    const providerActions = screen.getAllByRole('link', { name: /Check Patrol model/i });
    expect(providerActions.length).toBeGreaterThan(0);
    expect(providerActions[0]).toHaveAttribute('href', '/settings/pulse-intelligence/patrol');
    expect(screen.queryByRole('link', { name: 'Open Patrol settings' })).not.toBeInTheDocument();
  });

  it('does not repeat stale coverage caveats after a successful full patrol verified resources', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(
      defaultPatrolStatus({
        license_required: false,
        trust: {
          tracked: 3,
          currently_active: 1,
          resolved: 2,
          auto_resolved: 0,
          fix_verified: 0,
          fix_failed: 0,
          dismissed_as_noise: 0,
          dismissed_as_expected: 0,
          dismissed_as_later: 0,
          suppressed: 0,
          regressed_at_least_once: 2,
        },
      }),
    );
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
              'Recent Patrol activity only covered targeted checks and ended with errors. Run Patrol to check everything.',
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
    expect(
      screen.queryByText(/1 warning issue · 2 past regressions · health score 85\/100/),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol status')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Patrol trust summary header')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Patrol trust summary')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details-toggle')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details')).not.toBeInTheDocument();

    expect(
      screen.queryByText(
        'Recent Patrol runs encountered errors, so the current health summary may be incomplete.',
      ),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Recent coverage is also incomplete/i)).not.toBeInTheDocument();
  });

  it('keeps recent activity mix out of the default Patrol status chrome', async () => {
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

    await waitFor(() => {
      expect(getPatrolRunHistoryMock).toHaveBeenCalled();
      expect(screen.getByRole('heading', { name: 'Patrol' })).toBeInTheDocument();
    });

    expect(screen.queryByText(/No active issues · health score 100\/100/)).not.toBeInTheDocument();
    expect(screen.queryByText('Patrol status')).not.toBeInTheDocument();
    expect(screen.queryByTestId('patrol-summary-details-toggle')).not.toBeInTheDocument();
    expect(screen.queryByText(/Recent activity mix:/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Trigger mode:/)).not.toBeInTheDocument();
  });

  it('treats a selected zero-finding run as an empty snapshot and uses effective scope ids', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    intelligenceState.findings = [];
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

    fireEvent.click(screen.getByRole('button', { name: 'History' }));
    fireEvent.click(screen.getByRole('button', { name: 'Select mocked run' }));

    await waitFor(() => {
      expect(screen.getAllByText(/Patrol run/i).length).toBeGreaterThan(0);
    });

    expect(screen.queryByText('Finding record unavailable')).not.toBeInTheDocument();

    expect(findingsPanelState.latestProps).toMatchObject({
      filterOverride: 'all',
      filterFindingIds: [],
      scopeResourceIds: ['expanded-a', 'expanded-b'],
      scopeResourceTypes: ['vm'],
      showScopeWarnings: true,
      runSnapshotId: 'run-empty',
    });
  });

  it('shows the selected Patrol run outcome before the finding list', async () => {
    hasFeatureMock.mockReturnValue(true);
    licenseStatusMock.mockReturnValue({ subscription_state: 'active' });
    getPatrolStatusMock.mockResolvedValue(defaultPatrolStatus({ license_required: false }));
    intelligenceState.findings = [];
    runHistoryState.selection = {
      id: 'run-provider-error',
      started_at: '2026-03-12T10:00:00Z',
      completed_at: '2026-03-12T10:58:00Z',
      duration_ms: 3_480_000,
      type: 'full',
      trigger_reason: 'manual',
      scope_resource_ids: [],
      effective_scope_resource_ids: [],
      scope_resource_types: [],
      resources_checked: 72,
      nodes_checked: 0,
      guests_checked: 72,
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
      findings_summary: 'Provider failed during analysis',
      finding_ids: ['runtime-provider'],
      error_count: 1,
      error_summary: 'Selected model does not support Patrol tools',
      error_detail:
        "agentic patrol failed: API error (404): No endpoints found that support the provided 'tool_choice' value.",
      status: 'error',
      tool_call_count: 0,
    };

    render(() => <AIIntelligence />);

    await waitFor(() => {
      expect(getPatrolStatusMock).toHaveBeenCalled();
      expect(findingsPanelState.latestProps).not.toBeNull();
    });

    fireEvent.click(screen.getByRole('button', { name: 'History' }));
    fireEvent.click(screen.getByRole('button', { name: 'Select mocked run' }));

    await waitFor(() => {
      expect(screen.getAllByText(/Patrol run/i).length).toBeGreaterThan(0);
    });

    expect(screen.getByText('Manual')).toBeInTheDocument();
    expect(screen.getByText('error')).toBeInTheDocument();
    expect(
      screen.getByText(
        /Checked 72 resources in 58m\. Patrol ended with a runtime issue: Selected model does not support Patrol tools/i,
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Check Patrol model' })).toHaveAttribute(
      'href',
      '/settings/pulse-intelligence/patrol',
    );
    expect(findingsPanelState.latestProps).toMatchObject({
      filterOverride: 'all',
      filterFindingIds: ['runtime-provider'],
      scopeResourceIds: [],
      runSnapshotId: 'run-provider-error',
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

    fireEvent.click(screen.getByRole('button', { name: 'History' }));
    fireEvent.click(screen.getByRole('button', { name: 'Select mocked run' }));

    await waitFor(() => {
      expect(screen.getAllByText(/Patrol run/i).length).toBeGreaterThan(0);
    });

    expect(screen.queryByText('Finding record unavailable')).not.toBeInTheDocument();
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
    intelligenceState.findings = [];
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

    fireEvent.click(screen.getByRole('button', { name: 'History' }));
    fireEvent.click(screen.getByRole('button', { name: 'Select mocked run' }));

    await waitFor(() => {
      expect(screen.getAllByText(/Patrol run/i).length).toBeGreaterThan(0);
    });

    expect(screen.getByText(/Finding record unavailable/)).toBeInTheDocument();

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
