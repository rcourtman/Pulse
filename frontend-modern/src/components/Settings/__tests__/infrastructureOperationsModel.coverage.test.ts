import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { AgentCapability } from '@/utils/agentCapabilityPresentation';
import type { UnifiedAgentRow, UnifiedAgentSurface } from '../infrastructureOperationsModel';
import {
  buildCommandsByPlatform,
  buildDefaultTokenName,
  getReconnectActionLabel,
  getRowReportingSummary,
  shellQuoteArg,
} from '../infrastructureOperationsModel';

const makeRow = (overrides: Partial<UnifiedAgentRow>): UnifiedAgentRow => ({
  rowKey: 'row-1',
  id: 'agent-1',
  name: 'node-a',
  capabilities: [],
  status: 'active',
  upgradePlatform: 'linux',
  scope: { label: 'Default', category: 'default' },
  installFlags: [],
  searchText: '',
  surfaces: [],
  ...overrides,
});

const surface = (label: string, kind: AgentCapability = 'agent'): UnifiedAgentSurface => ({
  key: kind,
  kind,
  label,
  detail: '',
});

describe('buildDefaultTokenName', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('formats the current UTC instant as "Agent YYYY-MM-DD HH-MM"', () => {
    vi.setSystemTime(new Date('2024-03-05T09:07:30.123Z'));
    expect(buildDefaultTokenName()).toBe('Agent 2024-03-05 09-07');
  });

  it('reflects a different instant and rewrites the "T" and ":" separators', () => {
    vi.setSystemTime(new Date('2024-12-31T23:59:00.000Z'));
    expect(buildDefaultTokenName()).toBe('Agent 2024-12-31 23-59');
  });
});

describe('shellQuoteArg', () => {
  it('wraps a plain token in single quotes', () => {
    expect(shellQuoteArg('abc')).toBe("'abc'");
  });

  it('wraps an empty string into an empty single-quoted arg', () => {
    expect(shellQuoteArg('')).toBe("''");
  });

  it('preserves spaces and double quotes without escaping them', () => {
    expect(shellQuoteArg('say "hi" now')).toBe("'say \"hi\" now'");
  });

  it('keeps shell metacharacters literal inside the single quotes', () => {
    expect(shellQuoteArg('a$b`c\\d')).toBe("'a$b`c\\d'");
  });

  it('escapes an embedded single quote via the concatenated close-reopen sequence', () => {
    expect(shellQuoteArg("it's")).toBe("'it'\"'\"'s'");
  });

  it('escapes a leading single quote', () => {
    expect(shellQuoteArg("'ab")).toBe("''\"'\"'ab'");
  });

  it('escapes every single quote when several appear', () => {
    expect(shellQuoteArg("a'b'c")).toBe("'a'\"'\"'b'\"'\"'c'");
  });
});

describe('getReconnectActionLabel', () => {
  it('returns the Docker label when docker is present', () => {
    expect(getReconnectActionLabel(makeRow({ capabilities: ['docker'] }))).toBe(
      'Allow Docker reconnect',
    );
  });

  it('prefers docker over kubernetes when both are present', () => {
    expect(getReconnectActionLabel(makeRow({ capabilities: ['kubernetes', 'docker'] }))).toBe(
      'Allow Docker reconnect',
    );
  });

  it('returns the Kubernetes label when only kubernetes is present', () => {
    expect(getReconnectActionLabel(makeRow({ capabilities: ['kubernetes'] }))).toBe(
      'Allow Kubernetes reconnect',
    );
  });

  it('falls back to the host label for host-only capabilities', () => {
    expect(getReconnectActionLabel(makeRow({ capabilities: ['agent'] }))).toBe(
      'Allow host reconnect',
    );
  });

  it('falls back to the host label when capabilities is empty', () => {
    expect(getReconnectActionLabel(makeRow({ capabilities: [] }))).toBe('Allow host reconnect');
  });
});

describe('getRowReportingSummary', () => {
  it('returns an empty string when there are no surfaces', () => {
    expect(getRowReportingSummary(makeRow({ surfaces: [] }))).toBe('');
  });

  it('lower-cases only the first surface label and wraps a single item', () => {
    expect(
      getRowReportingSummary(
        makeRow({ surfaces: [surface('Host telemetry', 'agent')] }),
      ),
    ).toBe('Pulse is receiving host telemetry from this item.');
  });

  it('joins two surfaces with "and" and leaves the non-first label cased', () => {
    expect(
      getRowReportingSummary(
        makeRow({
          surfaces: [surface('Host telemetry', 'agent'), surface('Docker runtime data', 'docker')],
        }),
      ),
    ).toBe('Pulse is receiving host telemetry and Docker runtime data from this item.');
  });

  it('joins three surfaces with an Oxford comma', () => {
    expect(
      getRowReportingSummary(
        makeRow({
          surfaces: [
            surface('Host telemetry', 'agent'),
            surface('Docker runtime data', 'docker'),
            surface('Kubernetes cluster data', 'kubernetes'),
          ],
        }),
      ),
    ).toBe(
      'Pulse is receiving host telemetry, Docker runtime data, and Kubernetes cluster data from this item.',
    );
  });

  it('keeps an empty first label empty rather than lower-casing nothing', () => {
    // sentenceCaseSurfaceLabel guards label.length === 0; an empty first label
    // therefore flows through unchanged, producing a double-space gap.
    expect(
      getRowReportingSummary(
        makeRow({ surfaces: [surface('', 'agent')] }),
      ),
    ).toBe('Pulse is receiving  from this item.');
  });

  it('does not mutate labels that are not in the first position', () => {
    expect(
      getRowReportingSummary(
        makeRow({ surfaces: [surface('Host telemetry', 'agent'), surface('PBS data', 'pbs')] }),
      ),
    ).toBe('Pulse is receiving host telemetry and PBS data from this item.');
  });
});

describe('buildCommandsByPlatform', () => {
  const sections = buildCommandsByPlatform('UNIX_CMD', 'WIN_INTERACTIVE', 'WIN_PARAM');

  it('returns one section per AgentPlatform key', () => {
    expect(Object.keys(sections).sort()).toEqual(['freebsd', 'linux', 'macos', 'windows']);
  });

  it('routes the unix command through Linux with a single snippet', () => {
    expect(sections.linux.title).toBe('Install on Linux');
    expect(sections.linux.snippets).toHaveLength(1);
    expect(sections.linux.snippets[0].label).toBe('Install');
    expect(sections.linux.snippets[0].command).toBe('UNIX_CMD');
  });

  it('routes the same unix command through macOS with a distinct launchd framing', () => {
    expect(sections.macos.title).toBe('Install on macOS');
    expect(sections.macos.snippets).toHaveLength(1);
    expect(sections.macos.snippets[0].label).toBe('Install with launchd');
    expect(sections.macos.snippets[0].command).toBe('UNIX_CMD');
    expect(sections.macos.description).not.toBe(sections.linux.description);
  });

  it('routes the same unix command through FreeBSD with an rc.d framing', () => {
    expect(sections.freebsd.title).toBe('Install on FreeBSD / pfSense / OPNsense');
    expect(sections.freebsd.snippets).toHaveLength(1);
    expect(sections.freebsd.snippets[0].label).toBe('Install with rc.d');
    expect(sections.freebsd.snippets[0].command).toBe('UNIX_CMD');
  });

  it('uses both Windows commands in two distinct snippets', () => {
    expect(sections.windows.title).toBe('Install on Windows');
    expect(sections.windows.snippets).toHaveLength(2);
    expect(sections.windows.snippets[0].label).toBe('Install as Windows Service (PowerShell)');
    expect(sections.windows.snippets[0].command).toBe('WIN_INTERACTIVE');
    expect(sections.windows.snippets[1].label).toBe('Install with parameters (PowerShell)');
    expect(sections.windows.snippets[1].command).toBe('WIN_PARAM');
  });

  it('keeps every platform description distinct', () => {
    const descriptions = [
      sections.linux.description,
      sections.macos.description,
      sections.freebsd.description,
      sections.windows.description,
    ];
    expect(new Set(descriptions).size).toBe(descriptions.length);
  });
});
