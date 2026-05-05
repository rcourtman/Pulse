import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { MonitoredSystemImpactPreview } from '../MonitoredSystemImpactPreview';
import type { MonitoredSystemLedgerPreviewResponse } from '@/api/monitoredSystemLedger';

const buildPreview = (
  overrides: Partial<MonitoredSystemLedgerPreviewResponse> = {},
): MonitoredSystemLedgerPreviewResponse => ({
  current_count: 4,
  projected_count: 4,
  additional_count: 0,
  effect: 'no_change',
  current_systems: [],
  projected_systems: [],
  current_system: null,
  projected_system: null,
  ...overrides,
});

describe('MonitoredSystemImpactPreview', () => {
  afterEach(() => {
    cleanup();
  });

  it('describes unchanged usage when a preview has no count impact', () => {
    render(() => <MonitoredSystemImpactPreview preview={buildPreview()} />);

    expect(
      screen.getByText('This change keeps monitored-system count unchanged'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse currently counts 4 monitored systems. Saving this change would keep the count at 4 monitored systems.',
      ),
    ).toBeInTheDocument();
  });

  it('describes removed systems when a preview reduces monitored-system usage', () => {
    render(() => (
      <MonitoredSystemImpactPreview
        preview={buildPreview({
          current_count: 1,
          projected_count: 0,
          effect: 'removes_existing',
        })}
      />
    ));

    expect(screen.getByText('This change removes monitored systems')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse currently counts 1 monitored system. Saving this change would bring the count to 0 monitored systems (-1).',
      ),
    ).toBeInTheDocument();
  });

  it('describes added systems without capacity review copy', () => {
    render(() => (
      <MonitoredSystemImpactPreview
        preview={buildPreview({
          current_count: 9,
          projected_count: 11,
          additional_count: 2,
          effect: 'creates_multiple',
        })}
      />
    ));

    expect(screen.getByText('This change adds monitored systems')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse currently counts 9 monitored systems. Saving this change would bring the count to 11 monitored systems (+2).',
      ),
    ).toBeInTheDocument();
  });
});
