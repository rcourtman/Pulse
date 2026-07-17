import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { ThresholdsTableDockerUpdateAlertsSection } from '@/components/Alerts/ThresholdsTableDockerUpdateAlertsSection';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';
import { getAlertThresholdsDockerUpdatePresentation } from '@/utils/alertThresholdsPresentation';
import { FACTORY_DOCKER_DEFAULTS } from '@/utils/alertThresholdDefaults';

type DockerDefaultsShape = typeof FACTORY_DOCKER_DEFAULTS;

// The card owns the only UI lever for updateAlertDelayHours (#1545): the
// toggle must write -1 (off) — never 0, which the backend resets to 24 —
// and restore the previous positive delay when re-enabled.
function renderSection(initialDelayHours: number) {
  const [dockerDefaults, setDockerDefaultsSignal] = createSignal<DockerDefaultsShape>({
    ...FACTORY_DOCKER_DEFAULTS,
    updateAlertDelayHours: initialDelayHours,
  });
  const setHasUnsavedChanges = vi.fn();
  const setDockerDefaults = vi.fn((update: (prev: DockerDefaultsShape) => DockerDefaultsShape) => {
    setDockerDefaultsSignal((prev) => update(prev));
  });

  const props = {
    state: {
      dockerUpdatePresentation: getAlertThresholdsDockerUpdatePresentation(),
      dockerUpdateDelayInputId: 'docker-update-alert-delay',
    },
    tableProps: {
      get dockerDefaults() {
        return dockerDefaults();
      },
      setDockerDefaults,
      setHasUnsavedChanges,
    },
  } as unknown as ThresholdsTableSectionProps;

  const utils = render(() => <ThresholdsTableDockerUpdateAlertsSection {...props} />);
  return { ...utils, dockerDefaults, setHasUnsavedChanges };
}

afterEach(cleanup);

describe('ThresholdsTableDockerUpdateAlertsSection', () => {
  it('renders the delay input while enabled and hides it when off', () => {
    const { dockerDefaults } = renderSection(24);

    const input = screen.getByLabelText('Delay (hours)') as HTMLInputElement;
    expect(input.value).toBe('24');

    fireEvent.click(screen.getByRole('button', { pressed: true }));

    expect(dockerDefaults().updateAlertDelayHours).toBe(-1);
    expect(screen.queryByLabelText('Delay (hours)')).toBeNull();
  });

  it('saves -1 on toggle off and restores the prior delay on toggle on', () => {
    const { dockerDefaults, setHasUnsavedChanges } = renderSection(48);

    const toggle = screen.getByRole('button', { pressed: true });
    fireEvent.click(toggle);
    expect(dockerDefaults().updateAlertDelayHours).toBe(-1);

    fireEvent.click(toggle);
    expect(dockerDefaults().updateAlertDelayHours).toBe(48);
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
  });

  it('falls back to the factory delay when enabling from a persisted off state', () => {
    const { dockerDefaults } = renderSection(-1);

    expect(screen.queryByLabelText('Delay (hours)')).toBeNull();

    fireEvent.click(screen.getByRole('button', { pressed: false }));
    expect(dockerDefaults().updateAlertDelayHours).toBe(
      FACTORY_DOCKER_DEFAULTS.updateAlertDelayHours,
    );
  });

  it('clamps delay input edits to a positive whole number of hours', () => {
    const { dockerDefaults } = renderSection(24);

    const input = screen.getByLabelText('Delay (hours)') as HTMLInputElement;
    fireEvent.input(input, { target: { value: '0' } });
    // 0 would read as "unset" server-side and silently become 24; the input
    // clamps to the minimum positive delay instead.
    expect(dockerDefaults().updateAlertDelayHours).toBe(1);

    fireEvent.input(input, { target: { value: '72' } });
    expect(dockerDefaults().updateAlertDelayHours).toBe(72);
  });
});
