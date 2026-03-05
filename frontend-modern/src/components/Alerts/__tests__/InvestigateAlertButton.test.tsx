import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { Alert } from '@/types/api';

// ---------------------------------------------------------------------------
// Mocks — vi.hoisted ensures these are available before vi.mock factories run
// ---------------------------------------------------------------------------

const { openWithPromptMock, trackUpgradeClickedMock, formatAlertValueMock, mockAiChatStore } =
  vi.hoisted(() => {
    const openWithPromptMock = vi.fn();
    const trackUpgradeClickedMock = vi.fn();
    const formatAlertValueMock = vi.fn((value?: number, _type?: string) =>
      value !== undefined ? `${value.toFixed(1)}%` : 'N/A',
    );
    const mockAiChatStore = {
      enabled: true as boolean | null,
      openWithPrompt: (...args: unknown[]) => openWithPromptMock(...args),
    };
    return { openWithPromptMock, trackUpgradeClickedMock, formatAlertValueMock, mockAiChatStore };
  });

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: mockAiChatStore,
}));

vi.mock('@/stores/license', () => ({
  getUpgradeActionUrlOrFallback: (key: string) => `/pricing?feature=${key}`,
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackUpgradeClicked: (...args: unknown[]) => trackUpgradeClickedMock(...args),
}));

vi.mock('@/utils/alertFormatters', () => ({
  formatAlertValue: (...args: unknown[]) => formatAlertValueMock(...(args as [number, string])),
}));

// Import component AFTER mocks are set up
import { InvestigateAlertButton } from '../InvestigateAlertButton';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-1',
    type: 'cpu',
    level: 'warning',
    resourceId: 'vm-101',
    resourceName: 'test-vm',
    node: 'pve1',
    nodeDisplayName: 'PVE Node 1',
    instance: '',
    message: 'CPU usage is high',
    value: 82.5,
    threshold: 80,
    startTime: new Date(Date.now() - 5 * 60_000).toISOString(), // 5 mins ago
    acknowledged: false,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

beforeEach(() => {
  openWithPromptMock.mockReset();
  trackUpgradeClickedMock.mockReset();
  formatAlertValueMock.mockClear();
  mockAiChatStore.enabled = true;
  vi.spyOn(window, 'open').mockImplementation(() => null);
});

// ---------------------------------------------------------------------------
// Rendering / visibility
// ---------------------------------------------------------------------------
describe('InvestigateAlertButton', () => {
  describe('rendering and visibility', () => {
    it('renders nothing when AI is not enabled', () => {
      mockAiChatStore.enabled = false;
      const { container } = render(() => <InvestigateAlertButton alert={makeAlert()} />);
      expect(container.innerHTML).toBe('');
    });

    it('renders nothing when AI enabled is null', () => {
      mockAiChatStore.enabled = null;
      const { container } = render(() => <InvestigateAlertButton alert={makeAlert()} />);
      expect(container.innerHTML).toBe('');
    });

    it('renders a button when AI is enabled (default full variant)', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      const button = screen.getByRole('button');
      expect(button).toBeInTheDocument();
      expect(button).toHaveTextContent('Investigate with Pulse Assistant');
    });

    it('renders icon variant without text label', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" />);
      const button = screen.getByRole('button');
      expect(button).toBeInTheDocument();
      expect(button).not.toHaveTextContent('Investigate');
    });

    it('renders text variant with "Ask Pulse Assistant" label', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
      const button = screen.getByRole('button');
      expect(button).toHaveTextContent('Ask Pulse Assistant');
    });
  });

  // ---------------------------------------------------------------------------
  // Size classes
  // ---------------------------------------------------------------------------
  describe('size classes', () => {
    it('applies sm size classes to icon variant by default', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" />);
      const button = screen.getByRole('button');
      expect(button.className).toContain('w-6');
      expect(button.className).toContain('h-6');
    });

    it('applies sm SVG size when size="sm" is explicit', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" size="sm" />);
      const button = screen.getByRole('button');
      const svg = button.querySelector('svg')!;
      expect(svg.className.baseVal).toContain('w-3.5');
      expect(svg.className.baseVal).toContain('h-3.5');
    });

    it('applies md size classes when size="md"', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" size="md" />);
      const button = screen.getByRole('button');
      expect(button.className).toContain('w-8');
      expect(button.className).toContain('h-8');
      const svg = button.querySelector('svg')!;
      expect(svg.className.baseVal).toContain('w-4');
      expect(svg.className.baseVal).toContain('h-4');
    });
  });

  // ---------------------------------------------------------------------------
  // Custom class prop
  // ---------------------------------------------------------------------------
  describe('custom class', () => {
    it('appends custom class to button', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} class="my-custom-class" />);
      const button = screen.getByRole('button');
      expect(button.className).toContain('my-custom-class');
    });
  });

  // ---------------------------------------------------------------------------
  // License-locked state
  // ---------------------------------------------------------------------------
  describe('license-locked state', () => {
    it('sets aria-disabled when licenseLocked is true', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} licenseLocked={true} />);
      const button = screen.getByRole('button');
      expect(button).toHaveAttribute('aria-disabled', 'true');
    });

    it('shows locked title when licenseLocked', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} licenseLocked={true} />);
      const button = screen.getByRole('button');
      expect(button).toHaveAttribute(
        'title',
        'Pro required to investigate alerts with Pulse Assistant',
      );
    });

    it('shows unlocked title when not licenseLocked', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      const button = screen.getByRole('button');
      expect(button).toHaveAttribute('title', 'Ask Pulse Assistant to investigate this alert');
    });

    it('applies opacity class when locked', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} licenseLocked={true} />);
      const button = screen.getByRole('button');
      expect(button.className).toContain('opacity-60');
    });

    it('opens upgrade URL and tracks click when locked and clicked', async () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} licenseLocked={true} />);
      const button = screen.getByRole('button');
      await fireEvent.click(button);

      expect(trackUpgradeClickedMock).toHaveBeenCalledWith('investigate_alert_button', 'ai_alerts');
      expect(window.open).toHaveBeenCalledWith('/pricing?feature=ai_alerts', '_blank');
      expect(openWithPromptMock).not.toHaveBeenCalled();
    });

    it('still stops propagation when locked and clicked', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} licenseLocked={true} />);
      const button = screen.getByRole('button');

      const clickEvent = new MouseEvent('click', { bubbles: true, cancelable: true });
      const stopPropSpy = vi.spyOn(clickEvent, 'stopPropagation');
      const preventDefaultSpy = vi.spyOn(clickEvent, 'preventDefault');

      button.dispatchEvent(clickEvent);

      expect(stopPropSpy).toHaveBeenCalled();
      expect(preventDefaultSpy).toHaveBeenCalled();
    });
  });

  // ---------------------------------------------------------------------------
  // Click behavior (unlocked)
  // ---------------------------------------------------------------------------
  describe('click behavior (unlocked)', () => {
    it('calls openWithPrompt with alert context on click', async () => {
      const alert = makeAlert();
      render(() => <InvestigateAlertButton alert={alert} />);
      const button = screen.getByRole('button');
      await fireEvent.click(button);

      expect(openWithPromptMock).toHaveBeenCalledTimes(1);
      const [prompt, context] = openWithPromptMock.mock.calls[0] as [
        string,
        Record<string, unknown>,
      ];

      // Verify prompt contains key alert details
      expect(prompt).toContain('WARNING');
      expect(prompt).toContain('test-vm');
      expect(prompt).toContain('cpu');
      expect(prompt).toContain('PVE Node 1');

      // Verify context
      expect(context).toMatchObject({
        targetType: 'vm',
        targetId: 'vm-101',
        context: {
          alertId: 'alert-1',
          alertType: 'cpu',
          alertLevel: 'warning',
          alertMessage: 'CPU usage is high',
          guestName: 'test-vm',
          node: 'pve1',
        },
      });
    });

    it('does not call trackUpgradeClicked when unlocked', async () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      await fireEvent.click(screen.getByRole('button'));
      expect(trackUpgradeClickedMock).not.toHaveBeenCalled();
      expect(window.open).not.toHaveBeenCalled();
    });

    it('calls formatAlertValue for value and threshold', async () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      await fireEvent.click(screen.getByRole('button'));

      // formatAlertValue is called for both value and threshold
      expect(formatAlertValueMock).toHaveBeenCalledWith(82.5, 'cpu');
      expect(formatAlertValueMock).toHaveBeenCalledWith(80, 'cpu');
    });
  });

  // ---------------------------------------------------------------------------
  // Target type inference
  // ---------------------------------------------------------------------------
  describe('target type inference', () => {
    it('uses "node" when alert type starts with "node_"', async () => {
      const alert = makeAlert({ type: 'node_cpu' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('node');
    });

    it('uses "app-container" when alert type starts with "docker_"', async () => {
      const alert = makeAlert({ type: 'docker_cpu' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('app-container');
    });

    it('uses "storage" when alert type starts with "storage_"', async () => {
      const alert = makeAlert({ type: 'storage_usage' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('storage');
    });

    it('maps legacy resourceType aliases to canonical v6 target types', async () => {
      const alert = makeAlert({ type: 'memory' });
      render(() => <InvestigateAlertButton alert={alert} resourceType="lxc" />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('system-container');
    });

    it('infers "vm" when no resourceType is provided', async () => {
      const alert = makeAlert({ type: 'cpu' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('vm');
    });

    it('defaults to "agent" when type cannot be inferred', async () => {
      const alert = makeAlert({ resourceId: 'unknown-resource-1' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('agent');
    });

    it('normalizes metadata.resourceType aliases to canonical v6 targets', async () => {
      const alert = makeAlert({
        resourceId: 'resource-xyz',
        metadata: { resourceType: 'k8s' },
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('k8s-cluster');
    });

    it('normalizes metadata.resourceType host alias to agent', async () => {
      const alert = makeAlert({
        resourceId: 'resource-xyz',
        metadata: { resourceType: 'host' },
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('agent');
    });

    it('type prefix overrides explicit unsupported resourceType', async () => {
      const alert = makeAlert({ type: 'node_memory' });
      render(() => <InvestigateAlertButton alert={alert} resourceType="lxc" />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect(context.targetType).toBe('node');
    });
  });

  // ---------------------------------------------------------------------------
  // Duration formatting in prompt
  // ---------------------------------------------------------------------------
  describe('duration formatting', () => {
    const FIXED_NOW = new Date('2026-03-02T12:00:00Z').getTime();

    beforeEach(() => {
      vi.useFakeTimers();
      vi.setSystemTime(FIXED_NOW);
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it('formats duration in minutes for short-lived alerts', async () => {
      const alert = makeAlert({
        startTime: new Date(FIXED_NOW - 15 * 60_000).toISOString(),
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [prompt] = openWithPromptMock.mock.calls[0] as [string];
      expect(prompt).toContain('15 mins');
    });

    it('formats 1 minute as singular "1 min"', async () => {
      const alert = makeAlert({
        startTime: new Date(FIXED_NOW - 1 * 60_000).toISOString(),
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [prompt] = openWithPromptMock.mock.calls[0] as [string];
      expect(prompt).toContain('1 min');
      expect(prompt).not.toContain('1 mins');
    });

    it('formats duration in hours and minutes for long-lived alerts', async () => {
      const alert = makeAlert({
        startTime: new Date(FIXED_NOW - 90 * 60_000).toISOString(),
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [prompt] = openWithPromptMock.mock.calls[0] as [string];
      expect(prompt).toContain('1h 30m');
    });

    it('formats 0 minutes for alerts that just started', async () => {
      const alert = makeAlert({
        startTime: new Date(FIXED_NOW - 10_000).toISOString(), // 10 seconds ago
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [prompt] = openWithPromptMock.mock.calls[0] as [string];
      expect(prompt).toContain('0 mins');
    });
  });

  // ---------------------------------------------------------------------------
  // Prompt content — node display name
  // ---------------------------------------------------------------------------
  describe('prompt content', () => {
    it('uses nodeDisplayName when available', async () => {
      const alert = makeAlert({ node: 'pve1', nodeDisplayName: 'My Node' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [prompt] = openWithPromptMock.mock.calls[0] as [string];
      expect(prompt).toContain('**Node:** My Node');
    });

    it('falls back to node ID when nodeDisplayName is absent', async () => {
      const alert = makeAlert({ node: 'pve2', nodeDisplayName: undefined });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [prompt] = openWithPromptMock.mock.calls[0] as [string];
      expect(prompt).toContain('**Node:** pve2');
    });

    it('omits node line when node is empty', async () => {
      const alert = makeAlert({ node: '' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [prompt] = openWithPromptMock.mock.calls[0] as [string];
      expect(prompt).not.toContain('**Node:**');
    });

    it('includes CRITICAL in prompt for critical alerts', async () => {
      const alert = makeAlert({ level: 'critical' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const [prompt] = openWithPromptMock.mock.calls[0] as [string];
      expect(prompt).toContain('CRITICAL');
    });

    it('passes vmid in context when provided', async () => {
      const alert = makeAlert();
      render(() => <InvestigateAlertButton alert={alert} vmid={101} />);
      await fireEvent.click(screen.getByRole('button'));

      const [, context] = openWithPromptMock.mock.calls[0] as [string, Record<string, unknown>];
      expect((context.context as Record<string, unknown>).vmid).toBe(101);
    });
  });

  // ---------------------------------------------------------------------------
  // Event propagation
  // ---------------------------------------------------------------------------
  describe('event propagation', () => {
    it('stops propagation and prevents default on click', async () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      const button = screen.getByRole('button');

      const clickEvent = new MouseEvent('click', { bubbles: true, cancelable: true });
      const stopPropSpy = vi.spyOn(clickEvent, 'stopPropagation');
      const preventDefaultSpy = vi.spyOn(clickEvent, 'preventDefault');

      button.dispatchEvent(clickEvent);

      expect(stopPropSpy).toHaveBeenCalled();
      expect(preventDefaultSpy).toHaveBeenCalled();
    });
  });

  // ---------------------------------------------------------------------------
  // Hover behavior on full variant
  // ---------------------------------------------------------------------------
  describe('hover behavior', () => {
    it('shows arrow indicator on hover for full variant', async () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      const button = screen.getByRole('button');

      // Before hover — no arrow
      expect(button.textContent).not.toContain('→');

      await fireEvent.mouseEnter(button);
      expect(button.textContent).toContain('→');

      await fireEvent.mouseLeave(button);
      expect(button.textContent).not.toContain('→');
    });
  });

  // ---------------------------------------------------------------------------
  // Variant-specific rendering for all three variants
  // ---------------------------------------------------------------------------
  describe('variant rendering', () => {
    it('icon variant renders SVG but no text span', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" />);
      const button = screen.getByRole('button');
      expect(button.querySelector('svg')).toBeTruthy();
      expect(button.querySelector('span')).toBeNull();
    });

    it('text variant renders SVG and text span', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" />);
      const button = screen.getByRole('button');
      expect(button.querySelector('svg')).toBeTruthy();
      expect(button.querySelector('span')).toBeTruthy();
      expect(button.querySelector('span')!.textContent).toBe('Ask Pulse Assistant');
    });

    it('full variant (default) renders SVG and text span', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      const button = screen.getByRole('button');
      expect(button.querySelector('svg')).toBeTruthy();
      const spans = button.querySelectorAll('span');
      expect(spans.length).toBeGreaterThanOrEqual(1);
      // First span is the main label
      expect(spans[0].textContent).toBe('Investigate with Pulse Assistant');
    });
  });
});
