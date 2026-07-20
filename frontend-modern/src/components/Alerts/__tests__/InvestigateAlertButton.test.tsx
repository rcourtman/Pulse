import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import type { Alert } from '@/types/api';
import { getPublicPricingUrl } from '@/utils/pricingHandoff';

// ---------------------------------------------------------------------------
// Mocks — vi.hoisted ensures these are available before vi.mock factories run
// ---------------------------------------------------------------------------

const {
  openMock,
  openUpgradeDestinationMock,
  formatAlertValueMock,
  mockAiChatStore,
  getUpgradeActionDestinationMock,
  getUpgradeActionUrlOrFallbackMock,
  presentationPolicyHidesUpgradePromptsMock,
  triggerPatrolRunMock,
  notificationStoreMock,
} = vi.hoisted(() => {
  const openMock = vi.fn();
  const openUpgradeDestinationMock = vi.fn();
  const formatAlertValueMock = vi.fn((value?: number, _type?: string) =>
    value !== undefined ? `${value.toFixed(1)}%` : 'N/A',
  );
  const getUpgradeActionDestinationMock = vi.fn();
  const getUpgradeActionUrlOrFallbackMock = vi.fn();
  const presentationPolicyHidesUpgradePromptsMock = vi.fn();
  const mockAiChatStore = {
    enabled: true as boolean | null,
    open: (...args: unknown[]) => openMock(...args),
  };
  const triggerPatrolRunMock = vi.fn();
  const notificationStoreMock = {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
  };
  return {
    openMock,
    openUpgradeDestinationMock,
    formatAlertValueMock,
    mockAiChatStore,
    getUpgradeActionDestinationMock,
    getUpgradeActionUrlOrFallbackMock,
    presentationPolicyHidesUpgradePromptsMock,
    triggerPatrolRunMock,
    notificationStoreMock,
  };
});

vi.mock('@/stores/aiChat', () => ({
  aiChatStore: mockAiChatStore,
}));

vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: (...args: unknown[]) => getUpgradeActionDestinationMock(...args),
  getUpgradeActionUrlOrFallback: (...args: unknown[]) => getUpgradeActionUrlOrFallbackMock(...args),
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => presentationPolicyHidesUpgradePromptsMock(),
}));

vi.mock('@/components/shared/useUpgradeNavigation', () => ({
  useUpgradeNavigation: () => openUpgradeDestinationMock,
}));

vi.mock('@/utils/alertFormatters', () => ({
  formatAlertValue: (...args: unknown[]) => formatAlertValueMock(...(args as [number, string])),
}));

vi.mock('@/api/patrol', () => ({
  triggerPatrolRun: (...args: unknown[]) => triggerPatrolRunMock(...(args as [unknown])),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: notificationStoreMock,
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

function openedContext(): Record<string, unknown> {
  return openMock.mock.calls[0]?.[0] as Record<string, unknown>;
}

function openedBriefing(): { statusLabel?: string; detailLines?: string[]; subject?: string } {
  return openedContext().briefing as {
    statusLabel?: string;
    detailLines?: string[];
    subject?: string;
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  setActiveLocale(DEFAULT_LOCALE);
});

beforeEach(() => {
  setActiveLocale(DEFAULT_LOCALE);
  openMock.mockReset();
  formatAlertValueMock.mockClear();
  openUpgradeDestinationMock.mockReset();
  getUpgradeActionDestinationMock.mockReset();
  getUpgradeActionUrlOrFallbackMock.mockReset();
  presentationPolicyHidesUpgradePromptsMock.mockReset();
  mockAiChatStore.enabled = true;
  presentationPolicyHidesUpgradePromptsMock.mockReturnValue(false);
  getUpgradeActionDestinationMock.mockReturnValue({
    href: getPublicPricingUrl('ai_alerts'),
    external: true,
  });
  getUpgradeActionUrlOrFallbackMock.mockReturnValue(getPublicPricingUrl('ai_alerts'));
  vi.spyOn(window, 'open').mockImplementation(() => null);
  triggerPatrolRunMock.mockReset();
  triggerPatrolRunMock.mockResolvedValue({
    success: true,
    message: 'Triggered targeted Patrol check',
  });
  notificationStoreMock.success.mockReset();
  notificationStoreMock.error.mockReset();
  notificationStoreMock.warning.mockReset();
  notificationStoreMock.info.mockReset();
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
      expect(button).toHaveTextContent('Ask Pulse Assistant about this alert');
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
      expect(button).toHaveAttribute('title', 'Pro required to ask Pulse Assistant about alerts');
    });

    it('shows unlocked title when not licenseLocked', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      const button = screen.getByRole('button');
      expect(button).toHaveAttribute('title', 'Ask Pulse Assistant about this alert');
    });

    it('applies opacity class when locked', () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} licenseLocked={true} />);
      const button = screen.getByRole('button');
      expect(button.className).toContain('opacity-60');
    });

    it('opens upgrade URL when locked and clicked', async () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} licenseLocked={true} />);
      const button = screen.getByRole('button');
      await fireEvent.click(button);

      expect(openUpgradeDestinationMock).toHaveBeenCalledWith({
        href: getPublicPricingUrl('ai_alerts'),
        external: true,
      });
      expect(openMock).not.toHaveBeenCalled();
    });

    it('keeps the locked state non-promotional when upgrade prompts are hidden', async () => {
      presentationPolicyHidesUpgradePromptsMock.mockReturnValue(true);

      render(() => <InvestigateAlertButton alert={makeAlert()} licenseLocked={true} />);
      const button = screen.getByRole('button');

      expect(button).toHaveAttribute(
        'title',
        'Pulse Assistant alert help is not available for this alert',
      );

      await fireEvent.click(button);

      expect(openUpgradeDestinationMock).not.toHaveBeenCalled();
      expect(openMock).not.toHaveBeenCalled();
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
    it('calls open with alert context on click', async () => {
      const alert = makeAlert();
      render(() => <InvestigateAlertButton alert={alert} />);
      const button = screen.getByRole('button');
      await fireEvent.click(button);

      expect(openMock).toHaveBeenCalledTimes(1);
      const context = openedContext();

      // Verify context
      expect(context).toMatchObject({
        targetType: 'vm',
        targetId: 'vm-101',
        autonomousMode: false,
        briefing: expect.objectContaining({
          sourceLabel: 'Pulse Alerts',
          title: 'Alert investigation attached',
          subject: 'Warning cpu on test-vm',
          statusLabel: expect.stringContaining('Warning alert'),
          detailLines: expect.arrayContaining([
            'Current value 82.5%; threshold 80.0%',
            'Node: PVE Node 1',
            'Message: CPU usage is high',
          ]),
          actionLabel: 'Investigate alert alert-1',
          safetyNote: 'Diagnostics and remediation require operator approval.',
        }),
        context: {
          alertIdentifier: 'alert-1',
          alertType: 'cpu',
          alertLevel: 'warning',
          alertMessage: 'CPU usage is high',
          guestName: 'test-vm',
          node: 'pve1',
        },
      });
    });

    it('keeps alert investigation handoffs approval-required and command-free', async () => {
      const alert = makeAlert();
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();

      expect(context.autonomousMode).toBe(false);
      expect(context.briefing).toMatchObject({
        sourceLabel: 'Pulse Alerts',
        title: 'Alert investigation attached',
      });
      expect(JSON.stringify(context)).not.toContain('Execute diagnostic commands if safe');
      expect(JSON.stringify(context.briefing)).not.toContain('systemctl');
    });

    it('localizes the visible briefing while preserving machine identifiers and source values', async () => {
      setActiveLocale('de');

      const alert = makeAlert({
        id: 'alert:vm-101:cpu',
        resourceName: 'db-vm-01',
        node: 'pve1',
        nodeDisplayName: 'PVE Node 1',
        type: 'cpu',
        message: 'CPU usage is high',
      });
      render(() => <InvestigateAlertButton alert={alert} variant="text" />);

      expect(screen.getByRole('button')).toHaveTextContent('Pulse Assistant fragen');

      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      const briefing = openedBriefing();

      expect(briefing).toMatchObject({
        sourceLabel: 'Pulse Alerts',
        title: 'Warnungsanalyse angehaengt',
        subject: 'Warnung cpu auf db-vm-01',
        actionLabel: 'Warnmeldung alert:vm-101:cpu untersuchen',
        safetyNote: 'Diagnosen und Behebungen erfordern die Freigabe durch einen Operator.',
      });
      expect(briefing.statusLabel).toContain('Warnung-Warnmeldung');
      expect(briefing.detailLines).toEqual(
        expect.arrayContaining([
          'Aktueller Wert 82.5%; Schwellwert 80.0%',
          'Knoten: PVE Node 1',
          'Meldung: CPU usage is high',
        ]),
      );
      expect(context.context).toMatchObject({
        alertIdentifier: 'alert:vm-101:cpu',
        alertType: 'cpu',
        alertMessage: 'CPU usage is high',
        guestName: 'db-vm-01',
        node: 'pve1',
      });
      expect(context.handoffContext).toContain('Alert Identifier: alert:vm-101:cpu');
      expect(context.handoffContext).toContain('Alert Level: Warning');
      expect(context.handoffContext).toContain('Resource: db-vm-01');
      expect(context.handoffContext).toContain('Message: CPU usage is high');
    });

    it('does not open the upgrade destination when unlocked', async () => {
      render(() => <InvestigateAlertButton alert={makeAlert()} />);
      await fireEvent.click(screen.getByRole('button'));
      expect(openUpgradeDestinationMock).not.toHaveBeenCalled();
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
    it('uses "agent" when alert type starts with "node_"', async () => {
      const alert = makeAlert({ type: 'node_cpu' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('agent');
    });

    it('uses "app-container" when alert type starts with "docker_"', async () => {
      const alert = makeAlert({ type: 'docker_cpu' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('app-container');
    });

    it('uses "storage" when alert type starts with "storage_"', async () => {
      const alert = makeAlert({ type: 'storage_usage' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('storage');
    });

    it('ignores legacy resourceType aliases and falls back to resource ID inference', async () => {
      const alert = makeAlert({ type: 'memory' });
      render(() => <InvestigateAlertButton alert={alert} resourceType="lxc" />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('vm');
    });

    it('infers "vm" when no resourceType is provided', async () => {
      const alert = makeAlert({ type: 'cpu' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('vm');
    });

    it('defaults to "agent" when type cannot be inferred', async () => {
      const alert = makeAlert({ resourceId: 'unknown-resource-1' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('agent');
    });

    it('infers canonical "pod" from Kubernetes pod-style resource IDs', async () => {
      const alert = makeAlert({ resourceId: 'k8s:cluster-a:pod:api-7f9d' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('pod');
    });

    it('infers canonical "pod" from pod-prefixed resource IDs', async () => {
      const alert = makeAlert({ resourceId: 'pod:cluster-a:default/api-7f9d' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('pod');
    });

    it('canonicalizes metadata.resourceType k8s alias to the k8s-cluster target', async () => {
      // metadata.resourceType is a trusted, canonicalized resolution layer
      // (explicit type → metadata type → resource ID → agent). Commit
      // 05abf0721 ("Add Kubernetes, TrueNAS, and vSphere alert targets")
      // replaced the old "bare k8s/kubernetes → undefined" guard with a real
      // alias table, so a bare 'k8s' now resolves to the cluster-level target.
      // See src/utils/__tests__/alertTargetTypes.test.ts for the unit contract.
      const alert = makeAlert({
        resourceId: 'resource-xyz',
        metadata: { resourceType: 'k8s' },
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('k8s-cluster');
    });

    it('normalizes metadata.resourceType host alias to agent', async () => {
      const alert = makeAlert({
        resourceId: 'resource-xyz',
        metadata: { resourceType: 'host' },
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('agent');
    });

    it('type prefix overrides explicit unsupported resourceType', async () => {
      const alert = makeAlert({ type: 'node_memory' });
      render(() => <InvestigateAlertButton alert={alert} resourceType="lxc" />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
      expect(context.targetType).toBe('agent');
    });
  });

  // ---------------------------------------------------------------------------
  // Duration formatting in attached context
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

      expect(openedBriefing().statusLabel).toContain('15 mins');
    });

    it('formats 1 minute as singular "1 min"', async () => {
      const alert = makeAlert({
        startTime: new Date(FIXED_NOW - 1 * 60_000).toISOString(),
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      expect(openedBriefing().statusLabel).toContain('1 min');
      expect(openedBriefing().statusLabel).not.toContain('1 mins');
    });

    it('formats duration in hours and minutes for long-lived alerts', async () => {
      const alert = makeAlert({
        startTime: new Date(FIXED_NOW - 90 * 60_000).toISOString(),
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      expect(openedBriefing().statusLabel).toContain('1h 30m');
    });

    it('formats 0 minutes for alerts that just started', async () => {
      const alert = makeAlert({
        startTime: new Date(FIXED_NOW - 10_000).toISOString(), // 10 seconds ago
      });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      expect(openedBriefing().statusLabel).toContain('0 mins');
    });
  });

  // ---------------------------------------------------------------------------
  // Attached-context content — node display name
  // ---------------------------------------------------------------------------
  describe('attached context content', () => {
    it('uses nodeDisplayName when available', async () => {
      const alert = makeAlert({ node: 'pve1', nodeDisplayName: 'My Node' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      expect(openedBriefing().detailLines).toContain('Node: My Node');
    });

    it('falls back to node ID when nodeDisplayName is absent', async () => {
      const alert = makeAlert({ node: 'pve2', nodeDisplayName: undefined });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      expect(openedBriefing().detailLines).toContain('Node: pve2');
    });

    it('omits node line when node is empty', async () => {
      const alert = makeAlert({ node: '' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      expect(openedBriefing().detailLines?.some((line) => line.startsWith('Node:'))).toBe(false);
    });

    it('includes Critical in context for critical alerts', async () => {
      const alert = makeAlert({ level: 'critical' });
      render(() => <InvestigateAlertButton alert={alert} />);
      await fireEvent.click(screen.getByRole('button'));

      expect(openedBriefing().subject).toContain('Critical');
    });

    it('passes vmid in context when provided', async () => {
      const alert = makeAlert();
      render(() => <InvestigateAlertButton alert={alert} vmid={101} />);
      await fireEvent.click(screen.getByRole('button'));

      const context = openedContext();
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
      expect(spans[0].textContent).toBe('Ask Pulse Assistant about this alert');
    });
  });
});

describe('InvestigateAlertButton patrolOption', () => {
  it('renders Patrol as the primary action with an Assistant explanation menu', () => {
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="text" patrolOption />);
    expect(screen.getAllByRole('button')).toHaveLength(2);
    expect(screen.getByRole('button', { name: /Have Patrol investigate/i })).toBeInTheDocument();
    const toggle = screen.getByRole('button', { name: 'More alert actions' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });

  it('omits the split when the alert has no resource', () => {
    const alert = makeAlert({ resourceId: '' });
    render(() => <InvestigateAlertButton alert={alert} variant="text" patrolOption />);
    expect(screen.getAllByRole('button')).toHaveLength(1);
  });

  it('omits the split when license-locked', () => {
    render(() => (
      <InvestigateAlertButton alert={makeAlert()} variant="text" patrolOption licenseLocked />
    ));
    expect(screen.getAllByRole('button')).toHaveLength(1);
  });

  it('keeps the icon variant single-purpose even with patrolOption', () => {
    render(() => <InvestigateAlertButton alert={makeAlert()} variant="icon" patrolOption />);
    expect(screen.getAllByRole('button')).toHaveLength(1);
  });

  it('triggers a scoped Patrol run from the primary action', async () => {
    const alert = makeAlert({
      id: 'alert-9',
      type: 'cpu',
      resourceId: 'vm-101',
      resourceName: 'test-vm',
    });
    render(() => <InvestigateAlertButton alert={alert} variant="text" patrolOption />);

    await fireEvent.click(screen.getByRole('button', { name: /Have Patrol investigate/i }));

    await waitFor(() => {
      expect(triggerPatrolRunMock).toHaveBeenCalledWith({
        resource_ids: ['vm-101'],
        alert_identifier: 'alert-9',
        alert_type: 'cpu',
        context: 'Manual targeted check from alert: cpu',
      });
    });
    await waitFor(() => {
      expect(notificationStoreMock.success).toHaveBeenCalledWith('Patrol is investigating test-vm');
    });
  });

  it('does not open the Assistant when the Patrol primary action is chosen', async () => {
    const alert = makeAlert({ resourceId: 'vm-101' });
    render(() => <InvestigateAlertButton alert={alert} variant="text" patrolOption />);

    await fireEvent.click(screen.getByRole('button', { name: /Have Patrol investigate/i }));

    await waitFor(() => expect(triggerPatrolRunMock).toHaveBeenCalled());
    expect(openMock).not.toHaveBeenCalled();
  });

  it('opens the Assistant only from the secondary explanation item', async () => {
    const alert = makeAlert({ resourceId: 'vm-101' });
    render(() => <InvestigateAlertButton alert={alert} variant="text" patrolOption />);

    await fireEvent.click(screen.getByRole('button', { name: 'More alert actions' }));
    await fireEvent.click(await screen.findByRole('menuitem', { name: /Explain with Assistant/i }));

    expect(openMock).toHaveBeenCalledTimes(1);
    expect(triggerPatrolRunMock).not.toHaveBeenCalled();
  });
});
