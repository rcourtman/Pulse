import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';
import { useEnhancedCPUBarState } from '@/components/Dashboard/useEnhancedCPUBarState';

afterEach(() => {
  cleanup();
});

describe('useEnhancedCPUBarState', () => {
  it('centralizes enhanced CPU bar derivations and tooltip state', async () => {
    let captured: ReturnType<typeof useEnhancedCPUBarState> | undefined;

    const Harness = () => {
      captured = useEnhancedCPUBarState({
        usage: 95,
        loadAverage: 2.345,
        anomaly: {
          resource_id: 'vm-100',
          resource_name: 'test-vm',
          resource_type: 'qemu',
          metric: 'cpu',
          current_value: 95,
          baseline_mean: 30,
          baseline_std_dev: 5,
          z_score: 11,
          severity: 'critical',
          description: 'CPU usage is abnormally high',
        },
      });
      return <div onMouseEnter={captured.handleMouseEnter} onMouseLeave={captured.handleMouseLeave} />;
    };

    const { container } = render(() => <Harness />);

    expect(captured).toBeDefined();
    expect(captured!.presentation().barWidth).toBe('95%');
    expect(captured!.presentation().displayLoadAverage).toBe('2.35');
    expect(captured!.presentation().tooltipUsageClass).toBe('text-red-400');
    expect(captured!.presentation().hasAnomaly).toBe(true);

    container.firstElementChild?.dispatchEvent(new MouseEvent('mouseenter', { bubbles: true }));
    expect(captured!.tooltipVisible()).toBe(true);

    container.firstElementChild?.dispatchEvent(new MouseEvent('mouseleave', { bubbles: true }));
    expect(captured!.tooltipVisible()).toBe(false);
  });
});

