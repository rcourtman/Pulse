import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ThresholdsTableSMARTDefaultsCard } from '../ThresholdsTableSMARTDefaultsCard';

afterEach(cleanup);

describe('ThresholdsTableSMARTDefaultsCard', () => {
  it('updates health, counter, and percentage defaults and marks the form dirty', async () => {
    const [agentDefaults, setAgentDefaults] = createSignal<Record<string, number | undefined>>({
      smartHealthFailure: 1,
      smartPending: 1,
      smartLifeWarning: 10,
    });
    const setHasUnsavedChanges = vi.fn();

    render(() => (
      <ThresholdsTableSMARTDefaultsCard
        state={{} as never}
        tableProps={
          {
            get agentDefaults() {
              return agentDefaults();
            },
            setAgentDefaults,
            setHasUnsavedChanges,
          } as never
        }
      />
    ));

    await fireEvent.click(screen.getByLabelText('Alert on failed SMART health status'));
    await fireEvent.input(screen.getByLabelText('Pending sectors threshold'), {
      target: { value: '4' },
    });
    await fireEvent.input(screen.getByLabelText('Life remaining warning percentage'), {
      target: { value: '25' },
    });

    expect(agentDefaults().smartHealthFailure).toBe(0);
    expect(agentDefaults().smartPending).toBe(4);
    expect(agentDefaults().smartLifeWarning).toBe(25);
    expect(setHasUnsavedChanges).toHaveBeenCalledTimes(3);
  });

  it('clamps negative counters and percentages above 100', async () => {
    const [agentDefaults, setAgentDefaults] = createSignal<Record<string, number | undefined>>({});

    render(() => (
      <ThresholdsTableSMARTDefaultsCard
        state={{} as never}
        tableProps={
          {
            get agentDefaults() {
              return agentDefaults();
            },
            setAgentDefaults,
            setHasUnsavedChanges: vi.fn(),
          } as never
        }
      />
    ));

    await fireEvent.input(screen.getByLabelText('Media errors threshold'), {
      target: { value: '-3' },
    });
    await fireEvent.input(screen.getByLabelText('NVMe spare warning percentage'), {
      target: { value: '120' },
    });

    expect(agentDefaults().smartMediaErrors).toBe(0);
    expect(agentDefaults().smartSpareWarning).toBe(100);
  });
});
