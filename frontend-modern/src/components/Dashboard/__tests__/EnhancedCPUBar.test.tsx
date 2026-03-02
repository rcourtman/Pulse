import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@solidjs/testing-library';
import { EnhancedCPUBar } from '../EnhancedCPUBar';
import type { AnomalyReport } from '@/types/aiIntelligence';

function makeAnomaly(overrides: Partial<AnomalyReport> = {}): AnomalyReport {
  return {
    resource_id: 'vm-100',
    resource_name: 'test-vm',
    resource_type: 'qemu',
    metric: 'cpu',
    current_value: 85,
    baseline_mean: 30,
    baseline_std_dev: 5,
    z_score: 11,
    severity: 'high',
    description: 'CPU usage is abnormally high',
    ...overrides,
  };
}

/** Select the bar fill element (absolutely-positioned div with inline width style). */
function getBarFill(container: HTMLElement): HTMLElement | null {
  return container.querySelector('.absolute.top-0.left-0[style]');
}

/** Find the tooltip bar trigger element within the render container. */
function getBarTrigger(container: HTMLElement): HTMLElement {
  const trigger = container.querySelector('.bg-surface-hover');
  if (!trigger) throw new Error('Bar trigger element not found');
  return trigger as HTMLElement;
}

/**
 * Find the tooltip usage value element (inside the portal, not the bar label).
 * The tooltip has a "Usage" label row followed by the value span with font-medium.
 */
function getTooltipUsageValue(): HTMLElement | null {
  // The tooltip renders via Portal to document.body.
  // Find the "Usage" label, then get the sibling value span.
  const usageLabel = screen.queryByText('Usage');
  if (!usageLabel) return null;
  const row = usageLabel.closest('.flex.justify-between');
  if (!row) return null;
  return row.querySelector('.font-medium');
}

afterEach(() => {
  cleanup();
});

describe('EnhancedCPUBar', () => {
  // ── Basic rendering ──────────────────────────────────────────────

  it('renders formatted usage percentage', () => {
    render(() => <EnhancedCPUBar usage={45} />);
    expect(screen.getByText('45%')).toBeInTheDocument();
  });

  it('renders 0% for zero usage', () => {
    render(() => <EnhancedCPUBar usage={0} />);
    expect(screen.getByText('0%')).toBeInTheDocument();
  });

  it('renders rounded percentage for fractional usage', () => {
    render(() => <EnhancedCPUBar usage={82.7} />);
    expect(screen.getByText('83%')).toBeInTheDocument();
  });

  it('renders 0% for very small usage (<0.5)', () => {
    render(() => <EnhancedCPUBar usage={0.3} />);
    expect(screen.getByText('0%')).toBeInTheDocument();
  });

  // ── Bar width ────────────────────────────────────────────────────

  it('sets bar width to usage percentage', () => {
    const { container } = render(() => <EnhancedCPUBar usage={65} />);
    const bar = getBarFill(container);
    expect(bar).toHaveStyle({ width: '65%' });
  });

  it('caps bar width at 100% when usage exceeds 100', () => {
    const { container } = render(() => <EnhancedCPUBar usage={150} />);
    const bar = getBarFill(container);
    expect(bar).toHaveStyle({ width: '100%' });
  });

  it('sets bar width to 0% for zero usage', () => {
    const { container } = render(() => <EnhancedCPUBar usage={0} />);
    const bar = getBarFill(container);
    expect(bar).toHaveStyle({ width: '0%' });
  });

  // ── Bar color classes (exact boundary tests: cpu warning=80, critical=90) ──

  it('uses normal color class for usage below warning threshold', () => {
    const { container } = render(() => <EnhancedCPUBar usage={79} />);
    const bar = getBarFill(container);
    expect(bar?.className).toContain('bg-metric-normal-bg');
  });

  it('uses warning color class at exactly the warning threshold (80)', () => {
    const { container } = render(() => <EnhancedCPUBar usage={80} />);
    const bar = getBarFill(container);
    expect(bar?.className).toContain('bg-metric-warning-bg');
  });

  it('uses warning color class between warning and critical thresholds', () => {
    const { container } = render(() => <EnhancedCPUBar usage={89} />);
    const bar = getBarFill(container);
    expect(bar?.className).toContain('bg-metric-warning-bg');
  });

  it('uses critical color class at exactly the critical threshold (90)', () => {
    const { container } = render(() => <EnhancedCPUBar usage={90} />);
    const bar = getBarFill(container);
    expect(bar?.className).toContain('bg-metric-critical-bg');
  });

  it('uses critical color class well above critical threshold', () => {
    const { container } = render(() => <EnhancedCPUBar usage={99} />);
    const bar = getBarFill(container);
    expect(bar?.className).toContain('bg-metric-critical-bg');
  });

  // ── Cores display ────────────────────────────────────────────────

  it('renders core count in parentheses when provided', () => {
    render(() => <EnhancedCPUBar usage={50} cores={8} />);
    expect(screen.getByText('(8)')).toBeInTheDocument();
  });

  it('does not render core count when not provided', () => {
    render(() => <EnhancedCPUBar usage={50} />);
    expect(screen.queryByText(/\(\d+\)/)).not.toBeInTheDocument();
  });

  it('renders core count of 1', () => {
    render(() => <EnhancedCPUBar usage={50} cores={1} />);
    expect(screen.getByText('(1)')).toBeInTheDocument();
  });

  // ── Anomaly indicator ────────────────────────────────────────────

  describe('anomaly indicator', () => {
    it('shows ratio text for high anomaly (>=2x baseline)', () => {
      // current_value 85 / baseline_mean 30 ≈ 2.83 → "2.8x"
      const anomaly = makeAnomaly({ current_value: 85, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={85} anomaly={anomaly} />);
      expect(screen.getByText('2.8x')).toBeInTheDocument();
    });

    it('shows double-arrow for moderate anomaly (1.5-2x baseline)', () => {
      // current_value 48 / baseline_mean 30 = 1.6 → "↑↑"
      const anomaly = makeAnomaly({ current_value: 48, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={48} anomaly={anomaly} />);
      expect(screen.getByText('↑↑')).toBeInTheDocument();
    });

    it('shows single-arrow for mild anomaly (<1.5x baseline)', () => {
      // current_value 38 / baseline_mean 30 ≈ 1.27 → "↑"
      const anomaly = makeAnomaly({ current_value: 38, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={38} anomaly={anomaly} />);
      expect(screen.getByText('↑')).toBeInTheDocument();
    });

    it('applies critical severity class (text-red-400)', () => {
      const anomaly = makeAnomaly({ severity: 'critical', current_value: 90, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={90} anomaly={anomaly} />);
      const indicator = screen.getByText('3.0x');
      expect(indicator).toHaveClass('text-red-400');
    });

    it('applies high severity class (text-orange-400)', () => {
      const anomaly = makeAnomaly({ severity: 'high', current_value: 60, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={60} anomaly={anomaly} />);
      const indicator = screen.getByText('2.0x');
      expect(indicator).toHaveClass('text-orange-400');
    });

    it('applies medium severity class (text-yellow-400)', () => {
      const anomaly = makeAnomaly({ severity: 'medium', current_value: 60, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={60} anomaly={anomaly} />);
      const indicator = screen.getByText('2.0x');
      expect(indicator).toHaveClass('text-yellow-400');
    });

    it('applies low severity class (text-blue-400)', () => {
      const anomaly = makeAnomaly({ severity: 'low', current_value: 60, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={60} anomaly={anomaly} />);
      const indicator = screen.getByText('2.0x');
      expect(indicator).toHaveClass('text-blue-400');
    });

    it('falls back to text-yellow-400 for unknown severity', () => {
      const anomaly = makeAnomaly({ severity: 'unknown', current_value: 60, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={60} anomaly={anomaly} />);
      const indicator = screen.getByText('2.0x');
      expect(indicator).toHaveClass('text-yellow-400');
    });

    it('has animate-pulse class on anomaly indicator', () => {
      const anomaly = makeAnomaly({ current_value: 60, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={60} anomaly={anomaly} />);
      const indicator = screen.getByText('2.0x');
      expect(indicator).toHaveClass('animate-pulse');
    });

    it('sets title attribute with anomaly description', () => {
      const anomaly = makeAnomaly({ description: 'CPU spike detected', current_value: 60, baseline_mean: 30 });
      render(() => <EnhancedCPUBar usage={60} anomaly={anomaly} />);
      const indicator = screen.getByText('2.0x');
      expect(indicator).toHaveAttribute('title', 'CPU spike detected');
    });

    it('does not render indicator when anomaly is null', () => {
      const { container } = render(() => <EnhancedCPUBar usage={50} anomaly={null} />);
      expect(container.querySelector('.animate-pulse')).not.toBeInTheDocument();
    });

    it('does not render indicator when anomaly is undefined', () => {
      const { container } = render(() => <EnhancedCPUBar usage={50} />);
      expect(container.querySelector('.animate-pulse')).not.toBeInTheDocument();
    });

    it('does not render indicator when baseline_mean is 0', () => {
      const anomaly = makeAnomaly({ baseline_mean: 0, current_value: 50 });
      const { container } = render(() => <EnhancedCPUBar usage={50} anomaly={anomaly} />);
      expect(container.querySelector('.animate-pulse')).not.toBeInTheDocument();
    });
  });

  // ── Tooltip ──────────────────────────────────────────────────────

  describe('tooltip', () => {
    it('shows "CPU Details" header on mouse enter', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={45} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText('CPU Details')).toBeInTheDocument();
    });

    it('shows usage value in tooltip', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={72} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText('Usage')).toBeInTheDocument();
      const tooltipValue = getTooltipUsageValue();
      expect(tooltipValue).not.toBeNull();
      expect(tooltipValue!).toHaveTextContent('72%');
    });

    it('hides tooltip on mouse leave', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={45} />);
      const bar = getBarTrigger(container);
      await fireEvent.mouseEnter(bar);
      expect(screen.getByText('CPU Details')).toBeInTheDocument();
      await fireEvent.mouseLeave(bar);
      expect(screen.queryByText('CPU Details')).not.toBeInTheDocument();
    });

    it('shows CPU model in tooltip when provided', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={45} model="AMD Ryzen 9 5950X" />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText('AMD Ryzen 9 5950X')).toBeInTheDocument();
    });

    it('does not show model section when model is not provided', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={45} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      // The tooltip header should be present but there should be no model text element
      // (the model renders in a specific 9px text-slate-400 div)
      const tooltipHeader = screen.getByText('CPU Details');
      const tooltipContainer = tooltipHeader.closest('.min-w-\\[160px\\]');
      expect(tooltipContainer).not.toBeNull();
      const modelElement = tooltipContainer!.querySelector('.text-\\[9px\\]');
      expect(modelElement).toBeNull();
    });

    it('shows load average in tooltip when provided', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={45} loadAverage={2.35} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText('Load (1m)')).toBeInTheDocument();
      expect(screen.getByText('2.35')).toBeInTheDocument();
    });

    it('formats load average to 2 decimal places', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={45} loadAverage={1.1} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.getByText('1.10')).toBeInTheDocument();
    });

    it('does not show load average when not provided', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={45} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      expect(screen.queryByText('Load (1m)')).not.toBeInTheDocument();
    });

    it('applies red text to usage >90% in tooltip', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={95} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      // Scope to the tooltip Usage row to avoid false-positive from bar label
      const tooltipValue = getTooltipUsageValue();
      expect(tooltipValue).not.toBeNull();
      expect(tooltipValue!).toHaveClass('text-red-400');
      expect(tooltipValue!).toHaveTextContent('95%');
    });

    it('does not apply red text to usage <=90% in tooltip', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={85} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      const tooltipValue = getTooltipUsageValue();
      expect(tooltipValue).not.toBeNull();
      expect(tooltipValue!).not.toHaveClass('text-red-400');
      expect(tooltipValue!).toHaveClass('text-base-content');
    });

    it('uses text-base-content for usage exactly 90% in tooltip', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={90} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      const tooltipValue = getTooltipUsageValue();
      expect(tooltipValue).not.toBeNull();
      expect(tooltipValue!).toHaveClass('text-base-content');
      expect(tooltipValue!).not.toHaveClass('text-red-400');
    });

    it('applies red text at usage boundary (91%)', async () => {
      const { container } = render(() => <EnhancedCPUBar usage={91} />);
      await fireEvent.mouseEnter(getBarTrigger(container));
      const tooltipValue = getTooltipUsageValue();
      expect(tooltipValue).not.toBeNull();
      expect(tooltipValue!).toHaveClass('text-red-400');
    });
  });
});
