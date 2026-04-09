import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { MonitoredSystemAdmissionPreview } from '../MonitoredSystemAdmissionPreview';
import type { MonitoredSystemLedgerPreviewResponse } from '@/api/monitoredSystemLedger';

const buildPreview = (
  overrides: Partial<MonitoredSystemLedgerPreviewResponse> = {},
): MonitoredSystemLedgerPreviewResponse => ({
  current_count: 4,
  projected_count: 4,
  additional_count: 0,
  limit: 10,
  would_exceed_limit: false,
  effect: 'no_change',
  current_systems: [],
  projected_systems: [],
  current_system: null,
  projected_system: null,
  ...overrides,
});

describe('MonitoredSystemAdmissionPreview', () => {
  afterEach(() => {
    cleanup();
  });

  it('describes unchanged usage when a preview has no capacity impact', () => {
    render(() => <MonitoredSystemAdmissionPreview preview={buildPreview()} />);

    expect(
      screen.getByText('This change reuses your current monitored-system capacity'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Current usage 4 / 10. Saving this change keeps usage at 4 / 10.'),
    ).toBeInTheDocument();
  });

  it('describes freed capacity when a preview reduces monitored-system usage', () => {
    render(() => (
      <MonitoredSystemAdmissionPreview
        preview={buildPreview({
          current_count: 1,
          projected_count: 0,
          effect: 'removes_existing',
        })}
      />
    ));

    expect(screen.getByText('This change frees monitored-system capacity')).toBeInTheDocument();
    expect(
      screen.getByText('Current usage 1 / 10. Saving this change would move usage to 0 / 10 (-1).'),
    ).toBeInTheDocument();
  });
});
