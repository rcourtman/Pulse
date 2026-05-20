import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AlertCooldownSection } from './AlertCooldownSection';
import { AlertEscalationSection } from './AlertEscalationSection';
import { AlertGroupingSection } from './AlertGroupingSection';
import { AlertQuietHoursSection } from './AlertQuietHoursSection';
import { AlertRecoverySection } from './AlertRecoverySection';
import type { CooldownConfig, EscalationConfig, GroupingConfig, QuietHoursConfig } from './types';

function makeQuietHours(overrides: Partial<QuietHoursConfig> = {}): QuietHoursConfig {
  return {
    enabled: true,
    start: '22:00',
    end: '07:00',
    timezone: 'UTC',
    days: {
      monday: true,
      tuesday: true,
      wednesday: true,
      thursday: true,
      friday: true,
      saturday: false,
      sunday: false,
    },
    suppress: {
      performance: true,
      storage: true,
      offline: false,
    },
    ...overrides,
  };
}

describe('Alert schedule sections', () => {
  afterEach(() => {
    cleanup();
  });

  it('associates visible labels with quiet-hours controls', () => {
    render(() => (
      <AlertQuietHoursSection
        quietHours={makeQuietHours()}
        quietHourSuppressOptions={[
          { key: 'performance', label: 'Performance', description: 'CPU and memory alerts' },
          { key: 'storage', label: 'Storage', description: 'Disk alerts' },
          { key: 'offline', label: 'Offline', description: 'Connectivity alerts' },
        ]}
        weekdaysOnly={true}
        weekendsOnly={false}
        setQuietHoursEnabled={vi.fn()}
        setQuietHoursStart={vi.fn()}
        setQuietHoursEnd={vi.fn()}
        setQuietHoursTimezone={vi.fn()}
        toggleQuietDay={vi.fn()}
        setQuietSuppressCategory={vi.fn()}
      />
    ));

    expect(screen.getByRole('button', { name: 'Quiet hours Enabled' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByLabelText('Start time')).toHaveAttribute('type', 'time');
    expect(screen.getByLabelText('End time')).toHaveAttribute('type', 'time');
    expect(screen.getByRole('combobox', { name: 'Timezone' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Monday' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Saturday' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('associates visible labels with cooldown controls', () => {
    const cooldown: CooldownConfig = { enabled: true, minutes: 30, maxAlerts: 3 };

    render(() => (
      <AlertCooldownSection
        cooldown={cooldown}
        setCooldownEnabled={vi.fn()}
        setCooldownMinutes={vi.fn()}
        setCooldownMaxAlerts={vi.fn()}
      />
    ));

    expect(screen.getByRole('button', { name: 'Alert cooldown Enabled' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('spinbutton', { name: 'Cooldown period' })).toBeInTheDocument();
    expect(screen.getByRole('spinbutton', { name: 'Max alerts / hour' })).toBeInTheDocument();
  });

  it('associates visible labels with grouping controls', () => {
    const grouping: GroupingConfig = { enabled: true, window: 10, byNode: true, byGuest: false };

    render(() => (
      <AlertGroupingSection
        grouping={grouping}
        setGroupingEnabled={vi.fn()}
        setGroupingWindow={vi.fn()}
        setGroupingByNode={vi.fn()}
        setGroupingByGuest={vi.fn()}
      />
    ));

    expect(screen.getByRole('button', { name: 'Smart grouping Enabled' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('slider', { name: 'Grouping window' })).toHaveAttribute(
      'aria-valuetext',
      '10 minutes',
    );
    expect(screen.getByRole('checkbox', { name: 'By node' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'By workload' })).toBeInTheDocument();
  });

  it('associates visible labels with escalation controls', () => {
    const escalation: EscalationConfig = {
      enabled: true,
      levels: [{ after: 15, notify: 'email' }],
    };

    render(() => (
      <AlertEscalationSection
        escalation={escalation}
        setEscalationEnabled={vi.fn()}
        setEscalationAfter={vi.fn()}
        setEscalationNotify={vi.fn()}
        removeEscalationLevel={vi.fn()}
        addEscalationLevel={vi.fn()}
      />
    ));

    expect(screen.getByRole('button', { name: 'Alert escalation Enabled' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('spinbutton', { name: 'After' })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Notify' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove escalation level' })).toBeInTheDocument();
  });

  it('associates the visible recovery heading with its status toggle', () => {
    render(() => (
      <AlertRecoverySection notifyOnResolve={false} setNotifyOnResolveEnabled={vi.fn()} />
    ));

    expect(screen.getByRole('button', { name: 'Recovery notifications Disabled' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });
});
