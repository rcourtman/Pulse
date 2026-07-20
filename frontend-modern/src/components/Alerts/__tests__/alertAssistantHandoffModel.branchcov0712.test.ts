import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import { buildAlertAssistantHandoff } from '../alertAssistantHandoffModel';

// `buildAlertAssistantHandoff` always populates briefing/context/handoffResources/
// handoffContext, but its declared return type borrows `AIChatContext`, where those
// fields are optional and `context` is `Record<string, unknown>`. The strict
// tsconfig therefore flags direct deep access as possibly-undefined. We mirror the
// concrete runtime shape in a local type and cast once so the assertions below stay
// precise (`toBe`/`toEqual`) and type-safe — mirroring the sibling
// incidentAssistantHandoffModel.branchcov2.test.ts convention.
interface StrictBriefing {
  sourceLabel: string;
  title: string;
  subject: string;
  statusLabel: string;
  detailLines: string[];
  actionLabel: string;
  safetyNote: string;
}

interface StrictAlertContext {
  alertIdentifier: string;
  alertType: string;
  alertLevel: string;
  alertMessage: string;
  guestName: string;
  node: string;
  vmid?: number;
}

interface StrictHandoffContext {
  targetType: string;
  targetId: string;
  autonomousMode: boolean;
  handoffContext: string;
  handoffResources: { id: string; name: string; type: string; node: string }[];
  briefing: StrictBriefing;
  context: StrictAlertContext;
}

interface StrictHandoff {
  context: StrictHandoffContext;
}

const DEFAULT_NOW = new Date('2026-05-07T10:05:00.000Z');

function makeAlert(overrides: Partial<Alert> = {}): Alert {
  return {
    id: 'alert-1',
    type: 'cpu',
    level: 'warning',
    resourceId: 'vm-101',
    resourceName: 'app-vm',
    node: 'pve1',
    nodeDisplayName: 'PVE Node 1',
    instance: '',
    message: 'CPU usage is high',
    value: 82.5,
    threshold: 80,
    startTime: '2026-05-07T10:00:00.000Z',
    acknowledged: false,
    ...overrides,
  };
}

function buildHandoff(
  overrides: Partial<Alert> = {},
  options: { now?: Date; resourceType?: string; vmid?: number } = {},
): StrictHandoff {
  return buildAlertAssistantHandoff({
    alert: makeAlert(overrides),
    now: options.now ?? DEFAULT_NOW,
    resourceType: options.resourceType,
    vmid: options.vmid,
  }) as unknown as StrictHandoff;
}

// handoffContext renders the model overloads of formatAlertLevel /
// formatAlertDuration (locale = DEFAULT_LOCALE 'en'); the briefing subject and
// statusLabel render the no-locale overloads, which resolve to the active ('en')
// locale in the jsdom test environment. Both surfaces therefore render English.

describe('formatAlertDuration (exercised via buildAlertAssistantHandoff)', () => {
  it('returns singular "1 min" for an exact 1-minute delta (durationMins === 1 true arm)', () => {
    const handoff = buildHandoff(
      { startTime: '2026-05-07T10:00:00.000Z' },
      { now: new Date('2026-05-07T10:01:00.000Z') },
    );
    expect(handoff.context.briefing.statusLabel).toBe('Warning alert · Active 1 min');
    expect(handoff.context.handoffContext).toContain('Duration: 1 min');
  });

  it('returns plural "5 mins" for a multi-minute sub-hour delta (durationMins === 1 false arm)', () => {
    const handoff = buildHandoff(
      { startTime: '2026-05-07T10:00:00.000Z' },
      { now: new Date('2026-05-07T10:05:00.000Z') },
    );
    expect(handoff.context.briefing.statusLabel).toBe('Warning alert · Active 5 mins');
    expect(handoff.context.handoffContext).toContain('Duration: 5 mins');
  });

  it('formats a >= 60min delta as "Xh Ym" (durationMins >= 60 arm)', () => {
    const handoff = buildHandoff(
      { startTime: '2026-05-07T10:00:00.000Z' },
      { now: new Date('2026-05-07T11:05:00.000Z') },
    );
    // 65 minutes elapsed -> floor(65/60)=1 hour, 65%60=5 minutes.
    expect(handoff.context.briefing.statusLabel).toBe('Warning alert · Active 1h 5m');
    expect(handoff.context.handoffContext).toContain('Duration: 1h 5m');
  });

  it('returns "0 mins" when now equals startTime (zero delta, plural arm)', () => {
    const handoff = buildHandoff(
      { startTime: '2026-05-07T10:00:00.000Z' },
      { now: new Date('2026-05-07T10:00:00.000Z') },
    );
    expect(handoff.context.briefing.statusLabel).toBe('Warning alert · Active 0 mins');
    expect(handoff.context.handoffContext).toContain('Duration: 0 mins');
  });

  it('clamps a negative (now before startTime) delta to "0 mins" via Math.max(0, ...)', () => {
    const handoff = buildHandoff(
      { startTime: '2026-05-07T10:05:00.000Z' },
      { now: new Date('2026-05-07T10:00:00.000Z') },
    );
    expect(handoff.context.briefing.statusLabel).toBe('Warning alert · Active 0 mins');
    expect(handoff.context.handoffContext).toContain('Duration: 0 mins');
  });

  it('returns "unknown duration" when startTime is unparseable (startedAt not finite)', () => {
    const handoff = buildHandoff(
      { startTime: 'not-a-valid-date' },
      { now: new Date('2026-05-07T10:05:00.000Z') },
    );
    expect(handoff.context.briefing.statusLabel).toBe('Warning alert · Active unknown duration');
    expect(handoff.context.handoffContext).toContain('Duration: unknown duration');
  });

  it('returns "unknown duration" when now is an invalid Date (nowMs not finite)', () => {
    const handoff = buildHandoff(
      { startTime: '2026-05-07T10:00:00.000Z' },
      { now: new Date('not-a-valid-date') },
    );
    expect(handoff.context.briefing.statusLabel).toBe('Warning alert · Active unknown duration');
    expect(handoff.context.handoffContext).toContain('Duration: unknown duration');
  });
});

describe('formatAlertLevel (exercised via buildAlertAssistantHandoff)', () => {
  it("renders 'Critical' for level='critical' (=== 'critical' true arm)", () => {
    const handoff = buildHandoff({ level: 'critical' });
    // subject template: '{level} {alertType} on {resourceName}'
    expect(handoff.context.briefing.subject).toBe('Critical cpu on app-vm');
    expect(handoff.context.briefing.statusLabel).toBe('Critical alert · Active 5 mins');
    // handoffContext renders the model-level label (DEFAULT_LOCALE='en' overload).
    expect(handoff.context.handoffContext).toContain('Alert Level: Critical');
    expect(handoff.context.context.alertLevel).toBe('critical');
  });

  it("renders 'Info' for level='info' (=== 'info' true arm; level cast past the type union)", () => {
    // Alert.level is typed 'warning' | 'critical'; the runtime String() comparison
    // in formatAlertLevel still admits 'info', so we cast to reach that branch.
    const handoff = buildHandoff({ level: 'info' as unknown as Alert['level'] });
    expect(handoff.context.briefing.subject).toBe('Info cpu on app-vm');
    expect(handoff.context.briefing.statusLabel).toBe('Info alert · Active 5 mins');
    expect(handoff.context.handoffContext).toContain('Alert Level: Info');
  });

  it("renders 'Warning' for level='warning' (default fallback arm)", () => {
    const handoff = buildHandoff({ level: 'warning' });
    expect(handoff.context.briefing.subject).toBe('Warning cpu on app-vm');
    expect(handoff.context.briefing.statusLabel).toBe('Warning alert · Active 5 mins');
    expect(handoff.context.handoffContext).toContain('Alert Level: Warning');
  });
});

describe('formatContextLine (exercised via buildAlertAssistantHandoff)', () => {
  it('omits the "Message:" line when message is null (value === null branch)', () => {
    const handoff = buildHandoff({ message: null as unknown as string });
    expect(handoff.context.handoffContext).not.toContain('Message:');
    // Briefing's message ternary (alert.message ? ...) also drops the line; with a
    // metric cpu alert only currentMetric + node detailLines remain.
    expect(handoff.context.briefing.detailLines).toEqual([
      'Current value 82.5%; threshold 80.0%',
      'Node: PVE Node 1',
    ]);
  });

  it('omits the "Message:" line when message is undefined (value === undefined branch)', () => {
    const handoff = buildHandoff({ message: undefined as unknown as string });
    expect(handoff.context.handoffContext).not.toContain('Message:');
    expect(handoff.context.briefing.detailLines).toEqual([
      'Current value 82.5%; threshold 80.0%',
      'Node: PVE Node 1',
    ]);
  });

  it('omits the "Message:" line when message is empty string (trim -> empty -> undefined)', () => {
    const handoff = buildHandoff({ message: '' });
    expect(handoff.context.handoffContext).not.toContain('Message:');
    expect(handoff.context.briefing.detailLines).toEqual([
      'Current value 82.5%; threshold 80.0%',
      'Node: PVE Node 1',
    ]);
  });

  it('drops the "Message:" context line when message is whitespace-only (trim -> empty -> undefined)', () => {
    const handoff = buildHandoff({ message: '   ' });
    // formatContextLine trims to empty -> filtered out of handoffContext.
    expect(handoff.context.handoffContext).not.toContain('Message:');
    // Asymmetry: the briefing detail.message path does NOT trim, so the raw
    // whitespace survives there as a literal "Message:    " line.
    expect(handoff.context.briefing.detailLines).toContain('Message:    ');
  });

  it('renders the message line for a non-empty message (happy path)', () => {
    const handoff = buildHandoff({ message: 'Disk almost full' });
    expect(handoff.context.handoffContext).toContain('Message: Disk almost full');
    expect(handoff.context.briefing.detailLines).toContain('Message: Disk almost full');
  });

  it('coerces a numeric message to a rendered "Message:" line (String(value) branch)', () => {
    // Cast message to a number: formatContextLine runs String(value).trim() ->
    // "42" -> "Message: 42". (resourceId can't be used here because the same value
    // is consumed earlier by resolveAlertTargetType, which assumes a string and
    // would throw before formatContextLine runs.)
    const handoff = buildHandoff({ message: 42 as unknown as string });
    expect(handoff.context.handoffContext).toContain('Message: 42');
    expect(handoff.context.briefing.detailLines).toContain('Message: 42');
  });
});

describe('buildAlertAssistantHandoff (node-label, metadata-chain & briefing ternaries)', () => {
  it('falls back to alert.node when nodeDisplayName is undefined (|| chain 2nd arm)', () => {
    const handoff = buildHandoff({ node: 'pve2', nodeDisplayName: undefined });
    expect(handoff.context.handoffContext).toContain('Node: pve2');
    expect(handoff.context.briefing.detailLines).toContain('Node: pve2');
    // handoffResources always carries the raw node regardless of label resolution.
    expect(handoff.context.handoffResources[0].node).toBe('pve2');
  });

  it("falls back to alert.node when nodeDisplayName is empty string (|| chain, '' falsy arm)", () => {
    const handoff = buildHandoff({ node: 'pve2', nodeDisplayName: '' });
    expect(handoff.context.handoffContext).toContain('Node: pve2');
    expect(handoff.context.briefing.detailLines).toContain('Node: pve2');
  });

  it('yields empty nodeLabel when node is falsy (outer ternary false arm) -> drops the Node line', () => {
    const handoff = buildHandoff({ node: '' });
    expect(handoff.context.handoffContext).not.toContain('Node:');
    expect(handoff.context.briefing.detailLines).not.toContain('Node:');
    expect(handoff.context.context.node).toBe('');
    expect(handoff.context.handoffResources[0].node).toBe('');
  });

  it('hits the metadata?.resourceType optional-chain false branch when metadata is undefined', () => {
    // makeAlert has no metadata by default; type='cpu' -> targetType inferred from
    // the 'vm-101' id via inferAlertTargetTypeFromResourceId.
    const handoff = buildHandoff({ type: 'cpu' });
    expect(handoff.context.targetType).toBe('vm');
    expect(handoff.context.context.alertType).toBe('cpu');
  });

  it("hits the typeof metadata?.resourceType !== 'string' false branch for a non-string value", () => {
    const handoff = buildHandoff({
      type: 'cpu',
      metadata: { resourceType: 5 },
    });
    // Non-string resourceType is ignored -> falls back to resourceId inference -> 'vm'.
    expect(handoff.context.targetType).toBe('vm');
  });

  it('omits the currentMetric detailLine for a state alert (hasMetricValues false -> briefing ternary false arm)', () => {
    const handoff = buildHandoff({ type: 'powered-off', value: 0, threshold: 0 });
    expect(handoff.context.briefing.detailLines).toEqual([
      'Node: PVE Node 1',
      'Message: CPU usage is high',
    ]);
    // currentMetric ternary false -> currentValue/thresholdValue are '' ->
    // formatContextLine trims them out of handoffContext too.
    expect(handoff.context.handoffContext).not.toContain('Current Value:');
    expect(handoff.context.handoffContext).not.toContain('Threshold:');
  });

  it('keeps the currentMetric detailLine for a metric alert (hasMetricValues true -> briefing ternary true arm)', () => {
    const handoff = buildHandoff({ type: 'cpu', value: 92.5, threshold: 80 });
    expect(handoff.context.briefing.detailLines).toContain('Current value 92.5%; threshold 80.0%');
    expect(handoff.context.handoffContext).toContain('Current Value: 92.5%');
    expect(handoff.context.handoffContext).toContain('Threshold: 80.0%');
  });

  it('leaves context.vmid undefined when vmid is not supplied', () => {
    const handoff = buildHandoff();
    // The source spreads `vmid` (shorthand) from the optional input, so the key
    // exists but holds undefined when the caller omits it.
    expect(handoff.context.context.vmid).toBeUndefined();
  });

  it('propagates vmid into context when supplied', () => {
    const handoff = buildHandoff({}, { vmid: 101 });
    expect(handoff.context.context.vmid).toBe(101);
  });
});
