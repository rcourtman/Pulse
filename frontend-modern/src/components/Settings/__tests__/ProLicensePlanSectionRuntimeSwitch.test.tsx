import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { Component } from 'solid-js';
import { ProLicensePlanSection } from '../ProLicensePlanSection';

const baseMismatchProps = () => ({
  activationSuccessSummary: null,
  commercialMigrationNotice: null,
  commercialPlanModel: { summary: [], details: [] },
  currentPlanSummary: {
    title: 'Pulse Pro',
    body: 'Pro license is active on the community runtime.',
    badgeClass: '',
    statusLabel: 'Active',
    unlockedFeaturesLabel: 'Included',
    unlockedFeatures: [],
    includedExtras: [],
    supplementalBadges: ['Pro runtime missing'],
    privateRuntimeAction: {
      actionLabel: 'Open Pulse Pro downloads',
      actionUrl: 'https://pulserelay.pro/download.html',
    },
  },
  entitlements: null,
  grandfatheredPriceNotice: null,
  hasLicenseDetails: true,
  loading: false,
  onReload: () => {},
  planComparisonSummary: { cards: [], action: null },
  planSelectionPrompt: null,
  planStatus: null,
  purchaseActivationNotice: null,
  purchaseActivationAction: null,
});

const renderInRouter = (component: Component) => {
  render(() => (
    <Router>
      <Route path="/" component={component} />
    </Router>
  ));
};

describe('ProLicensePlanSection in-app Pro runtime switch', () => {
  afterEach(() => cleanup());

  it('offers one-click in-place switch and shows progress and error states', () => {
    const onSwitch = vi.fn();
    renderInRouter(() => (
      <ProLicensePlanSection
        {...baseMismatchProps()}
        proRuntimeSwitchState={{
          info: {
            available: true,
            currentVersion: '6.8.0',
            version: '6.8.0',
            installInPlace: true,
          },
          loading: false,
          loadError: null,
          switching: false,
          status: null,
          error: null,
          onRefresh: () => {},
          onSwitch,
        }}
      />
    ));

    expect(screen.getByText('Switch this install to Pulse Pro')).toBeTruthy();
    expect(screen.getByText('v6.8.0')).toBeTruthy();
    const button = screen.getByRole('button', { name: 'Switch to Pulse Pro' });
    fireEvent.click(button);
    expect(onSwitch).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('link', { name: 'Open Pulse Pro downloads' })).toBeTruthy();
  });

  it('renders live switch progress bar while switching', () => {
    renderInRouter(() => (
      <ProLicensePlanSection
        {...baseMismatchProps()}
        proRuntimeSwitchState={{
          info: {
            available: true,
            currentVersion: '6.8.0',
            version: '6.8.0',
            installInPlace: true,
          },
          loading: false,
          loadError: null,
          switching: true,
          status: {
            status: 'extracting',
            progress: 40,
            message: 'Extracting update...',
            updatedAt: '2026-11-20T23:00:00Z',
          },
          error: null,
          onRefresh: () => {},
          onSwitch: () => {},
        }}
      />
    ));

    expect(screen.getByText('Extracting update...')).toBeTruthy();
    expect(screen.getByText('40%')).toBeTruthy();
    const progress = screen.getByRole('progressbar');
    expect(progress.getAttribute('aria-valuenow')).toBe('40');
    const button = screen.getByRole('button', { name: 'Switching to Pulse Pro...' });
    expect((button as HTMLButtonElement).disabled).toBe(true);
  });

  it('renders digest-pinned Docker switch commands without exposing secret keys', () => {
    const digest = 'sha256:' + 'cd'.repeat(32);
    const pinnedRef = `registry.pulserelay.pro/pulse/pulse-pro@${digest}`;
    renderInRouter(() => (
      <ProLicensePlanSection
        {...baseMismatchProps()}
        proRuntimeSwitchState={{
          info: {
            available: true,
            currentVersion: '6.8.0',
            version: '6.8.0',
            installInPlace: false,
            docker: {
              version: '6.8.0',
              image: 'registry.pulserelay.pro/pulse/pulse-pro',
              imageDigest: digest,
              loginCommand:
                "printf '%s' '<activation-key>' | docker login registry.pulserelay.pro -u 'lic_123' --password-stdin",
              composePullCommand: `PULSE_IMAGE='${pinnedRef}' docker compose pull`,
              composeUpCommand: `PULSE_IMAGE='${pinnedRef}' docker compose up -d`,
            },
          },
          loading: false,
          loadError: null,
          switching: false,
          status: null,
          error: null,
          onRefresh: () => {},
          onSwitch: () => {},
        }}
      />
    ));

    expect(screen.getByText('Switch this container to Pulse Pro')).toBeTruthy();
    expect(screen.getByText(`PULSE_IMAGE='${pinnedRef}' docker compose pull`)).toBeTruthy();
    expect(screen.getByText(`PULSE_IMAGE='${pinnedRef}' docker compose up -d`)).toBeTruthy();
    expect(
      screen.getByText("docker login registry.pulserelay.pro --username 'lic_123'"),
    ).toBeTruthy();
    expect(screen.queryByText(/--password-stdin/)).toBeNull();
  });

  it('renders tailored refusal copy and retry option when switch is unavailable', () => {
    const onRefresh = vi.fn();
    renderInRouter(() => (
      <ProLicensePlanSection
        {...baseMismatchProps()}
        proRuntimeSwitchState={{
          info: {
            available: false,
            reason: 'unavailable',
            currentVersion: '6.8.0',
            installInPlace: false,
          },
          loading: false,
          loadError: null,
          switching: false,
          status: null,
          error: null,
          onRefresh,
          onSwitch: () => {},
        }}
      />
    ));

    expect(screen.getByText(/Could not reach the Pulse Pro download service/)).toBeTruthy();
    const retryBtn = screen.getByRole('button', { name: 'Check again' });
    fireEvent.click(retryBtn);
    expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('link', { name: 'Open Pulse Pro downloads' })).toBeTruthy();
  });

  it('explains older_release refusal without offering an in-place button', () => {
    renderInRouter(() => (
      <ProLicensePlanSection
        {...baseMismatchProps()}
        proRuntimeSwitchState={{
          info: {
            available: false,
            reason: 'older_release',
            currentVersion: '6.9.0',
            version: '6.8.0',
            installInPlace: false,
          },
          loading: false,
          loadError: null,
          switching: false,
          status: null,
          error: null,
          onRefresh: () => {},
          onSwitch: () => {},
        }}
      />
    ));

    expect(screen.getByText(/older than this install/i)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Switch to Pulse Pro' })).toBeNull();
  });
});
