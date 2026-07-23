import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { TelemetryPingPreview } from '@/api/settings';
import type { SettingsTab } from '../settingsNavigationModel';

type UseSystemSettingsStateModule = typeof import('../useSystemSettingsState');

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

const buildTelemetryPreviewPayload = (
  overrides: Partial<TelemetryPingPreview> = {},
): TelemetryPingPreview => ({
  schema_version: 2,
  sent_at: '2026-07-23T08:30:00Z',
  install_id: 'preview-install-id',
  version: '6.0.0',
  version_channel: 'stable',
  version_is_development: false,
  version_is_published_release: true,
  platform: 'docker',
  os: 'linux',
  arch: 'amd64',
  event: 'heartbeat',
  deployment_method: 'docker_compose',
  known_install_age_bucket: '1_7d',
  activation_stage: 'monitoring',
  time_to_first_monitored_resource_bucket: 'under_15m',
  estate_size_bucket: '1_10',
  auth_configured: true,
  configured_connections: 1,
  monitoring_active: true,
  outcome_observed_30d: false,
  pve_nodes: 1,
  pbs_instances: 0,
  pmg_instances: 0,
  vms: 2,
  containers: 3,
  agent_hosts: 1,
  docker_hosts: 0,
  docker_containers: 0,
  kubernetes_clusters: 0,
  kubernetes_nodes: 0,
  kubernetes_pods: 0,
  kubernetes_deployments: 0,
  storage_pools: 1,
  physical_disks: 2,
  ceph_clusters: 0,
  network_shares: 0,
  truenas_systems: 0,
  truenas_vms: 0,
  truenas_apps: 0,
  vmware_hosts: 0,
  vmware_vms: 0,
  vmware_datastores: 0,
  availability_targets: 1,
  ai_enabled: false,
  patrol_enabled: false,
  discovery_enabled: false,
  notifications_enabled: false,
  ai_actions_enabled: false,
  active_alerts: 0,
  relay_enabled: false,
  sso_enabled: false,
  multi_tenant: false,
  paid_license: false,
  has_api_tokens: true,
  update_attempts_30d: 0,
  update_successes_30d: 0,
  update_failures_30d: 0,
  update_last_failure_category: undefined,
  alerts_fired_30d: 0,
  alerts_acknowledged_30d: 0,
  alerts_resolved_30d: 0,
  notification_attempts_7d: 0,
  notification_deliveries_7d: 0,
  notification_failures_7d: 0,
  pulse_intelligence_loop_configured: false,
  pulse_intelligence_loop_active_30d: false,
  pulse_intelligence_complete_operations_loop_30d: false,
  pulse_intelligence_approved_execution_loop_30d: false,
  pulse_intelligence_resolved_operations_loop_30d: false,
  pulse_intelligence_patrol_control_completed_operations_loop_30d: false,
  pulse_intelligence_patrol_control_resolved_operations_loop_30d: false,
  pulse_intelligence_patrol_control_paid_completed_operations_loop_30d: false,
  pulse_intelligence_patrol_control_paid_resolved_operations_loop_30d: false,
  pulse_intelligence_pro_activation_completed_operations_loop_30d: false,
  pulse_intelligence_pro_activation_resolved_operations_loop_30d: false,
  pulse_intelligence_pro_activation_paid_completed_operations_loop_30d: false,
  pulse_intelligence_pro_activation_paid_resolved_operations_loop_30d: false,
  pulse_intelligence_governed_action_active_30d: false,
  pulse_intelligence_assistant_operations_loop_30d: false,
  pulse_intelligence_assistant_approved_execution_loop_30d: false,
  pulse_intelligence_assistant_approved_action_success_loop_30d: false,
  pulse_intelligence_assistant_resolved_operations_loop_30d: false,
  pulse_intelligence_external_agent_operations_loop_30d: false,
  pulse_intelligence_external_agent_approved_execution_loop_30d: false,
  pulse_intelligence_external_agent_approved_action_success_loop_30d: false,
  pulse_intelligence_external_agent_resolved_operations_loop_30d: false,
  pulse_intelligence_mcp_adapter_operations_loop_30d: false,
  pulse_intelligence_mcp_adapter_approved_execution_loop_30d: false,
  pulse_intelligence_mcp_adapter_approved_action_success_loop_30d: false,
  pulse_intelligence_mcp_adapter_resolved_operations_loop_30d: false,
  pulse_intelligence_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_assistant_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_patrol_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_patrol_control_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_pro_activation_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_mcp_operations_loop_starter_requests_30d: 0,
  pulse_intelligence_assistant_ai_calls_30d: 0,
  pulse_intelligence_assistant_context_ai_calls_30d: 0,
  pulse_intelligence_assistant_tool_calls_30d: 0,
  pulse_intelligence_patrol_ai_calls_30d: 0,
  pulse_intelligence_patrol_runs_30d: 0,
  pulse_intelligence_patrol_new_findings_30d: 0,
  pulse_intelligence_patrol_investigations_30d: 0,
  pulse_intelligence_patrol_resolved_findings_30d: 0,
  pulse_intelligence_patrol_autofixes_30d: 0,
  pulse_intelligence_external_agent_enabled: false,
  pulse_intelligence_external_agent_used_30d: false,
  pulse_intelligence_mcp_adapter_used_30d: false,
  pulse_intelligence_external_agent_context_requests_30d: 0,
  pulse_intelligence_external_agent_event_stream_requests_30d: 0,
  pulse_intelligence_external_agent_provisioning_requests_30d: 0,
  pulse_intelligence_external_agent_operator_state_requests_30d: 0,
  pulse_intelligence_external_agent_finding_requests_30d: 0,
  pulse_intelligence_external_agent_action_requests_30d: 0,
  pulse_intelligence_action_plans_30d: 0,
  pulse_intelligence_approval_requests_30d: 0,
  pulse_intelligence_rejected_action_decisions_30d: 0,
  pulse_intelligence_approved_action_decisions_30d: 0,
  pulse_intelligence_approved_action_attempts_30d: 0,
  pulse_intelligence_approved_action_successes_30d: 0,
  pulse_intelligence_approved_action_failures_pre_dispatch_30d: 0,
  pulse_intelligence_approved_action_failures_execution_30d: 0,
  pulse_intelligence_approved_action_failures_unverified_30d: 0,
  pulse_intelligence_approved_action_stuck_executing_30d: 0,
  pulse_intelligence_approved_action_last_failure_reason_30d: undefined,
  ...overrides,
});

describe('useSystemSettingsState', () => {
  let useSystemSettingsState: UseSystemSettingsStateModule['useSystemSettingsState'];
  let getSystemSettingsMock: ReturnType<typeof vi.fn>;
  let getTelemetryPreviewMock: ReturnType<typeof vi.fn>;
  let resetTelemetryInstallIDMock: ReturnType<typeof vi.fn>;
  let getVersionMock: ReturnType<typeof vi.fn>;
  let updateSystemSettingsMock: ReturnType<typeof vi.fn>;
  let updateStoreVersionInfoMock: ReturnType<typeof vi.fn>;
  let copyToClipboardMock: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vi.resetModules();

    getSystemSettingsMock = vi.fn();
    getTelemetryPreviewMock = vi.fn();
    resetTelemetryInstallIDMock = vi.fn();
    getVersionMock = vi.fn().mockResolvedValue({
      version: '1.0.0',
      build: 'test',
      runtime: 'go1.22',
      channel: 'stable',
      isDocker: false,
      isSourceBuild: false,
      isDevelopment: false,
      deploymentType: 'systemd',
    });
    updateSystemSettingsMock = vi.fn().mockResolvedValue(undefined);
    updateStoreVersionInfoMock = vi.fn().mockReturnValue(null);
    copyToClipboardMock = vi.fn().mockResolvedValue(true);

    vi.doMock('@/api/settings', () => ({
      SettingsAPI: {
        getSystemSettings: getSystemSettingsMock,
        getTelemetryPreview: getTelemetryPreviewMock,
        resetTelemetryInstallID: resetTelemetryInstallIDMock,
        updateSystemSettings: updateSystemSettingsMock,
      },
    }));

    vi.doMock('@/api/updates', () => ({
      UpdatesAPI: {
        getVersion: getVersionMock,
        getUpdatePlan: vi.fn(),
      },
    }));

    vi.doMock('@/stores/notifications', () => ({
      notificationStore: {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
      },
    }));

    vi.doMock('@/utils/logger', () => ({
      logger: {
        error: vi.fn(),
        warn: vi.fn(),
        info: vi.fn(),
        debug: vi.fn(),
      },
    }));

    vi.doMock('@/utils/clipboard', () => ({
      copyToClipboard: copyToClipboardMock,
    }));

    vi.doMock('@/utils/apiClient', () => ({
      apiFetch: vi.fn(),
      apiFetchJSON: vi.fn(),
    }));

    vi.doMock('@/stores/updates', () => ({
      updateStore: {
        checkForUpdates: vi.fn().mockResolvedValue(undefined),
        applyUpdate: vi.fn().mockResolvedValue(true),
        updateInfo: vi.fn().mockReturnValue(null),
        versionInfo: updateStoreVersionInfoMock,
        isDismissed: vi.fn().mockReturnValue(false),
        clearDismissed: vi.fn(),
      },
    }));

    vi.doMock('@/stores/systemSettings', () => ({
      updateDockerUpdateActionsSetting: vi.fn(),
    }));

    ({ useSystemSettingsState } = await import('../useSystemSettingsState'));
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.resetModules();
  });

  const mountHook = () => {
    let dispose = () => {};
    let hookState: ReturnType<UseSystemSettingsStateModule['useSystemSettingsState']>;

    createRoot((d) => {
      dispose = d;
      const [_discoveryEnabled, setDiscoveryEnabled] = createSignal(false);

      hookState = useSystemSettingsState({
        activeTab: () => 'system-updates' as SettingsTab,
        loadSecurityStatus: async () => {},
        setDiscoveryEnabled,
        applySavedDiscoverySubnet: () => {},
      });
    });

    return { dispose, hookState: hookState! };
  };

  const mountHookWithTab = (tab: SettingsTab) => {
    let dispose = () => {};
    let hookState: ReturnType<UseSystemSettingsStateModule['useSystemSettingsState']>;

    createRoot((d) => {
      dispose = d;
      const [_discoveryEnabled, setDiscoveryEnabled] = createSignal(false);

      hookState = useSystemSettingsState({
        activeTab: () => tab,
        loadSecurityStatus: async () => {},
        setDiscoveryEnabled,
        applySavedDiscoverySubnet: () => {},
      });
    });

    return { dispose, hookState: hookState! };
  };

  it('shows clean success toast without port reference when saving from General tab', async () => {
    const { hookState, dispose } = mountHookWithTab('system-general');
    const { notificationStore } = await import('@/stores/notifications');

    await hookState.saveSettings();
    await flushAsync();

    expect(notificationStore.success).toHaveBeenCalledWith('Settings saved successfully.');
    dispose();
  });

  it('shows network changes toast when saving from Network tab', async () => {
    const { hookState, dispose } = mountHookWithTab('system-network');
    const { notificationStore } = await import('@/stores/notifications');

    await hookState.saveSettings();
    await flushAsync();

    expect(notificationStore.success).toHaveBeenCalledWith(
      'Settings saved successfully. Service restart may be required for network changes.',
    );
    dispose();
  });

  it('does not reload the page when saving from General tab', async () => {
    const savedLocation = window.location;
    vi.useFakeTimers();
    try {
      const reloadMock = vi.fn();
      Object.defineProperty(window, 'location', {
        value: { ...savedLocation, reload: reloadMock },
        writable: true,
        configurable: true,
      });

      const { hookState, dispose } = mountHookWithTab('system-general');

      await hookState.saveSettings();
      await flushAsync();

      vi.advanceTimersByTime(5000);
      expect(reloadMock).not.toHaveBeenCalled();

      dispose();
    } finally {
      Object.defineProperty(window, 'location', {
        value: savedLocation,
        writable: true,
        configurable: true,
      });
      vi.useRealTimers();
    }
  });

  it('reloads the page when saving from Network tab', async () => {
    const savedLocation = window.location;
    vi.useFakeTimers();
    try {
      const reloadMock = vi.fn();
      Object.defineProperty(window, 'location', {
        value: { ...savedLocation, reload: reloadMock },
        writable: true,
        configurable: true,
      });

      const { hookState, dispose } = mountHookWithTab('system-network');

      await hookState.saveSettings();
      await flushAsync();

      vi.advanceTimersByTime(3000);
      expect(reloadMock).toHaveBeenCalledOnce();

      dispose();
    } finally {
      Object.defineProperty(window, 'location', {
        value: savedLocation,
        writable: true,
        configurable: true,
      });
      vi.useRealTimers();
    }
  });

  it('normalizes persisted RC auto-updates off during initialization', async () => {
    getSystemSettingsMock.mockResolvedValue({
      updateChannel: 'rc',
      autoUpdateEnabled: true,
    });

    const { hookState, dispose } = mountHook();

    await hookState.initializeSystemSettingsState();
    await flushAsync();

    expect(hookState.updateChannel()).toBe('rc');
    expect(hookState.autoUpdateEnabled()).toBe(false);
    dispose();
  });

  it('reuses version info from the shared update store during initialization', async () => {
    updateStoreVersionInfoMock.mockReturnValue({
      version: '1.0.1',
      build: 'retry',
      runtime: 'go1.22',
      channel: 'stable',
      isDocker: false,
      isSourceBuild: false,
      isDevelopment: false,
      deploymentType: 'systemd',
    });

    const { hookState, dispose } = mountHook();

    await hookState.initializeSystemSettingsState();
    await flushAsync();

    expect(hookState.versionInfo()?.version).toBe('1.0.1');
    expect(getVersionMock).not.toHaveBeenCalled();
    dispose();
  });

  it('forces autoUpdateEnabled off when saving RC channel settings', async () => {
    const { hookState, dispose } = mountHook();

    hookState.setUpdateChannel('rc');
    hookState.setAutoUpdateEnabled(true);

    await hookState.saveSettings();
    await flushAsync();

    expect(updateSystemSettingsMock).toHaveBeenCalledWith(
      expect.objectContaining({
        updateChannel: 'rc',
        autoUpdateEnabled: false,
      }),
    );
    dispose();
  });

  it('loads the telemetry preview payload on demand', async () => {
    getTelemetryPreviewMock.mockResolvedValue({
      enabled: true,
      payload: buildTelemetryPreviewPayload(),
    });

    const { hookState, dispose } = mountHookWithTab('system-general');

    await hookState.handleLoadTelemetryPreview();
    await flushAsync();

    expect(getTelemetryPreviewMock).toHaveBeenCalledOnce();
    expect(hookState.telemetryPreviewPayload()).toContain('"install_id": "preview-install-id"');
    expect(hookState.telemetryPreviewPayload()).toContain(
      '"pulse_intelligence_patrol_control_completed_operations_loop_30d": false',
    );
    expect(hookState.telemetryPreviewEnabled()).toBe(true);
    dispose();
  });

  it('copies the loaded telemetry preview payload', async () => {
    getTelemetryPreviewMock.mockResolvedValue({
      enabled: false,
      payload: buildTelemetryPreviewPayload(),
    });

    const { hookState, dispose } = mountHookWithTab('system-general');
    const { notificationStore } = await import('@/stores/notifications');

    await hookState.handleLoadTelemetryPreview();
    await hookState.handleCopyTelemetryPreview();
    await flushAsync();

    expect(copyToClipboardMock).toHaveBeenCalledWith(
      expect.stringContaining('"pulse_intelligence_resolved_operations_loop_30d": false'),
    );
    expect(notificationStore.success).toHaveBeenCalledWith(
      'Telemetry payload copied to clipboard',
      2000,
    );
    dispose();
  });

  it('resets the telemetry install ID after confirmation', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    resetTelemetryInstallIDMock.mockResolvedValue({
      enabled: true,
      payload: buildTelemetryPreviewPayload({
        install_id: 'rotated-install-id',
        platform: 'binary',
        pve_nodes: 0,
        vms: 0,
        containers: 0,
        agent_hosts: 0,
        storage_pools: 0,
        physical_disks: 0,
        availability_targets: 0,
      }),
    });

    const { hookState, dispose } = mountHookWithTab('system-general');
    const { notificationStore } = await import('@/stores/notifications');

    await hookState.handleResetTelemetryInstallID();
    await flushAsync();

    expect(confirmSpy).toHaveBeenCalled();
    expect(resetTelemetryInstallIDMock).toHaveBeenCalledOnce();
    expect(hookState.telemetryPreviewPayload()).toContain('"install_id": "rotated-install-id"');
    expect(notificationStore.success).toHaveBeenCalledWith('Telemetry install ID reset', 3000);

    confirmSpy.mockRestore();
    dispose();
  });
});
