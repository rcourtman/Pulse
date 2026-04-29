import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { DiagnosticsResultsPanel } from '@/components/Settings/DiagnosticsResultsPanel';
import type { DiagnosticsData } from '@/components/Settings/diagnosticsModel';

describe('DiagnosticsResultsPanel', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders the commercial funnel card with readable breakdown labels', () => {
    const diagnosticsData: DiagnosticsData = {
      version: '6.0.0',
      runtime: 'go',
      uptime: 3600,
      nodes: [],
      pbs: [],
      system: {
        os: 'linux',
        arch: 'amd64',
        goVersion: 'go1.25',
        numCPU: 8,
        numGoroutine: 32,
        memoryMB: 128,
      },
      commercialFunnel: {
        enabled: true,
        status: 'active',
        windowDays: 30,
        summary: {
          pricing_viewed: 3,
          paywall_viewed: 0,
          trial_started: 1,
          upgrade_clicked: 0,
          checkout_clicked: 2,
          checkout_started: 2,
          checkout_completed: 1,
          license_activated: 1,
          license_activation_failed: 0,
          period: {
            from: '2026-03-19T00:00:00Z',
            to: '2026-04-18T00:00:00Z',
          },
        },
        daily: [
          {
            day: '2026-04-17',
            pricing_viewed: 1,
            paywall_viewed: 0,
            trial_started: 0,
            upgrade_clicked: 0,
            checkout_clicked: 1,
            checkout_started: 1,
            checkout_completed: 0,
            license_activated: 0,
            license_activation_failed: 0,
          },
          {
            day: '2026-04-18',
            pricing_viewed: 2,
            paywall_viewed: 0,
            trial_started: 1,
            upgrade_clicked: 0,
            checkout_clicked: 1,
            checkout_started: 1,
            checkout_completed: 1,
            license_activated: 1,
            license_activation_failed: 0,
          },
        ],
        surfaces: [
          {
            key: 'settings_self_hosted_billing_compare_prompt',
            pricing_viewed: 0,
            paywall_viewed: 0,
            trial_started: 0,
            upgrade_clicked: 0,
            checkout_clicked: 2,
            checkout_started: 0,
            checkout_completed: 0,
            license_activated: 0,
            license_activation_failed: 0,
          },
        ],
        capabilities: [
          {
            key: 'self_hosted_plan',
            pricing_viewed: 3,
            paywall_viewed: 0,
            trial_started: 0,
            upgrade_clicked: 0,
            checkout_clicked: 2,
            checkout_started: 2,
            checkout_completed: 1,
            license_activated: 1,
            license_activation_failed: 0,
          },
        ],
        notes: ['Local pricing and activation events show at least one completed conversion.'],
      },
      infrastructureOnboarding: {
        enabled: true,
        status: 'warning',
        windowDays: 30,
        summary: {
          opened: 4,
          api_path_selected: 2,
          agent_path_selected: 1,
          probe_detected: 1,
          probe_no_match: 2,
          probe_error: 0,
          catalog_selected: 2,
          credentials_opened: 1,
          period: {
            from: '2026-03-19T00:00:00Z',
            to: '2026-04-18T00:00:00Z',
          },
        },
        daily: [
          {
            day: '2026-04-17',
            opened: 2,
            api_path_selected: 1,
            agent_path_selected: 1,
            probe_detected: 0,
            probe_no_match: 1,
            probe_error: 0,
            catalog_selected: 1,
            credentials_opened: 0,
          },
          {
            day: '2026-04-18',
            opened: 2,
            api_path_selected: 1,
            agent_path_selected: 0,
            probe_detected: 1,
            probe_no_match: 1,
            probe_error: 0,
            catalog_selected: 1,
            credentials_opened: 1,
          },
        ],
        paths: [
          { key: 'api', count: 2 },
          { key: 'agent', count: 1 },
        ],
        platforms: [
          {
            key: 'truenas',
            catalog_selected: 2,
            credentials_opened: 1,
          },
        ],
        notes: ['More probed addresses miss than detect a supported API-backed platform.'],
      },
      dockerAgents: {
        agentsOnline: 1,
        agentsTotal: 2,
        agentsReportingVersion: 2,
        agentsWithTokenBinding: 1,
        agentsWithoutTokenBinding: 1,
        agentsNeedingAttention: 1,
      },
      errors: [],
    };

    render(() => (
      <DiagnosticsResultsPanel
        diagnosticsData={diagnosticsData}
        loading={false}
        onRunDiagnostics={() => {}}
      />
    ));

    expect(screen.getByText('Commercial Funnel')).toBeInTheDocument();
    expect(screen.getByText('Pricing Views')).toBeInTheDocument();
    expect(screen.getByText('Checkout Clicks')).toBeInTheDocument();
    expect(screen.getByText('Self Hosted Plan')).toBeInTheDocument();
    expect(screen.getByText('Settings Self Hosted Billing Compare Prompt')).toBeInTheDocument();
    expect(screen.getByText(/Pricing 3/i)).toBeInTheDocument();
    expect(screen.getByText('Infrastructure Onboarding')).toBeInTheDocument();
    expect(screen.getByText('Credentials Opened')).toBeInTheDocument();
    expect(screen.getByText('TrueNAS SCALE')).toBeInTheDocument();
    expect(screen.getByText('API')).toBeInTheDocument();
    expect(screen.getByText('Docker / Podman agents')).toBeInTheDocument();
    expect(screen.getByText('Agent-backed Docker / Podman monitoring')).toBeInTheDocument();
    expect(screen.queryByText('Container Runtime Agents')).not.toBeInTheDocument();
  });
});
