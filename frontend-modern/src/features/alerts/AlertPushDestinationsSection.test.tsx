import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AlertPushDestinationsSection } from './AlertPushDestinationsSection';
import {
  ALERT_DESTINATIONS_PUSH_GATE_MESSAGE,
  ALERT_DESTINATIONS_PUSH_MINIMUM_SEVERITY_HELP,
  ALERT_DESTINATIONS_PUSH_READY_MESSAGE,
  ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL,
} from '@/utils/alertDestinationsPresentation';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

const upgradeDestination: UpgradeDestination = {
  href: '/settings/pulse-intelligence/billing',
  external: false,
};

function renderWithRouter(ui: () => JSX.Element) {
  return render(() => (
    <Router>
      <Route path="/" component={ui} />
    </Router>
  ));
}

describe('AlertPushDestinationsSection', () => {
  afterEach(() => {
    cleanup();
  });

  it('points licensed installs at Remote Access settings', () => {
    const onMinimumSeverityChange = vi.fn();
    renderWithRouter(() => (
      <AlertPushDestinationsSection
        relayLicensed={true}
        showUpgradePrompts={true}
        upgradeDestination={upgradeDestination}
        minimumSeverity="critical"
        onMinimumSeverityChange={onMinimumSeverityChange}
      />
    ));

    expect(screen.getByText(ALERT_DESTINATIONS_PUSH_READY_MESSAGE)).toBeInTheDocument();
    const link = screen.getByRole('link', {
      name: `${ALERT_DESTINATIONS_PUSH_SETUP_LINK_LABEL} →`,
    });
    expect(link).toHaveAttribute('href', '/settings/system-relay');

    const severity = screen.getByRole('combobox', { name: 'Minimum alert severity' });
    expect(severity).toHaveValue('critical');
    expect(
      screen.queryByRole('option', { name: 'Warnings and critical alerts' }),
    ).not.toBeInTheDocument();
    fireEvent.change(severity, { target: { value: 'all' } });
    expect(onMinimumSeverityChange).toHaveBeenCalledWith('all');
    expect(screen.getByText(ALERT_DESTINATIONS_PUSH_MINIMUM_SEVERITY_HELP)).toBeInTheDocument();
  });

  it('shows the Relay upgrade gate to unlicensed installs', () => {
    renderWithRouter(() => (
      <AlertPushDestinationsSection
        relayLicensed={false}
        showUpgradePrompts={true}
        upgradeDestination={upgradeDestination}
      />
    ));

    expect(screen.getByText(ALERT_DESTINATIONS_PUSH_GATE_MESSAGE)).toBeInTheDocument();
    expect(screen.queryByText(ALERT_DESTINATIONS_PUSH_READY_MESSAGE)).not.toBeInTheDocument();
  });

  it('hides the upgrade call-to-action when upgrade prompts are suppressed', () => {
    renderWithRouter(() => (
      <AlertPushDestinationsSection
        relayLicensed={false}
        showUpgradePrompts={false}
        upgradeDestination={upgradeDestination}
      />
    ));

    expect(screen.getByText(ALERT_DESTINATIONS_PUSH_GATE_MESSAGE)).toBeInTheDocument();
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });
});
