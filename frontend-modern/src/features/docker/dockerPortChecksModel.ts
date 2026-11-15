import type { DockerPortCheckSuggestion, DockerPortChecksResult } from '@/api/availabilityTargets';

// The Docker page offers an availability check for each published container
// port that has none. This module holds the wording and the bookkeeping so the
// notice component stays a thin view.

const DISMISSED_STORAGE_KEY = 'pulse.dockerPortChecks.dismissed';

export const dockerPortCheckKey = (suggestion: DockerPortCheckSuggestion): string =>
  `${suggestion.hostId}/${suggestion.containerName}/${suggestion.privatePort}`;

const plural = (count: number, one: string, many: string): string => (count === 1 ? one : many);

/** Ports Pulse can check from outside their host. */
export const checkablePortSuggestions = (
  suggestions: DockerPortCheckSuggestion[],
): DockerPortCheckSuggestion[] => suggestions.filter((suggestion) => !suggestion.skipReason);

/** Ports worth offering: checkable and not already dismissed in this browser. */
export const offeredPortSuggestions = (
  suggestions: DockerPortCheckSuggestion[],
  dismissed: ReadonlySet<string>,
): DockerPortCheckSuggestion[] =>
  checkablePortSuggestions(suggestions).filter(
    (suggestion) => !dismissed.has(dockerPortCheckKey(suggestion)),
  );

export const portOfferMessage = (count: number): string =>
  `${count} published container ${plural(count, 'port has', 'ports have')} no availability check.`;

const portLabel = (suggestion: DockerPortCheckSuggestion): string =>
  `${suggestion.containerName} ${suggestion.publicPort}`;

/**
 * One line per Docker host, naming the address Pulse will connect to:
 * "docker-vm at 192.168.1.20: web 8081, dns 5380 (192.168.1.21)". The address
 * comes from the agent's report, so it is shown before anything is added.
 */
export const portLinesByHost = (suggestions: DockerPortCheckSuggestion[]): string[] => {
  const byHost = new Map<string, { name: string; ports: DockerPortCheckSuggestion[] }>();
  for (const suggestion of suggestions) {
    const entry = byHost.get(suggestion.hostId) ?? {
      name: suggestion.hostName || suggestion.hostId,
      ports: [],
    };
    entry.ports.push(suggestion);
    byHost.set(suggestion.hostId, entry);
  }
  return [...byHost.values()].map((entry) => {
    const counts = new Map<string, number>();
    for (const port of entry.ports) {
      if (port.address) counts.set(port.address, (counts.get(port.address) ?? 0) + 1);
    }
    const hostAddress = [...counts.entries()].sort((a, b) => b[1] - a[1])[0]?.[0];
    const ports = entry.ports.map((port) =>
      port.address && port.address !== hostAddress
        ? `${portLabel(port)} (${port.address})`
        : portLabel(port),
    );
    return `${entry.name}${hostAddress ? ` at ${hostAddress}` : ''}: ${ports.join(', ')}`;
  });
};

/** Why some published ports are not offered, in the user's terms. */
export const skippedPortLines = (suggestions: DockerPortCheckSuggestion[]): string[] => {
  const withReason = (reason: string) =>
    suggestions.filter((suggestion) => suggestion.skipReason === reason);
  const loopback = withReason('loopback_only');
  const noAddress = withReason('no_host_address');
  const notHTTP = withReason('not_http');
  const lines: string[] = [];
  if (notHTTP.length > 0) {
    lines.push(
      `${plural(notHTTP.length, 'Not a web port', 'Not web ports')}, so not offered: ${notHTTP.map(portLabel).join(', ')}.`,
    );
  }
  if (loopback.length > 0) {
    lines.push(
      `Published on 127.0.0.1 only, so Pulse cannot reach ${plural(loopback.length, 'it', 'them')}: ${loopback.map(portLabel).join(', ')}.`,
    );
  }
  if (noAddress.length > 0) {
    lines.push(`Pulse has no address for the host yet: ${noAddress.map(portLabel).join(', ')}.`);
  }
  return lines;
};

export type PortChecksResultSummary = {
  tone: 'success' | 'info';
  message: string;
  unansweredLine?: string;
};

export const summarizePortChecksResult = (
  result: DockerPortChecksResult,
): PortChecksResultSummary => {
  const created = result.created.length;
  const unanswered = result.unanswered.length;
  const unansweredLine =
    unanswered > 0
      ? `${unanswered} ${plural(unanswered, 'port', 'ports')} did not answer over HTTP, so no check was added: ${result.unanswered.map(portLabel).join(', ')}.`
      : undefined;
  if (created === 0) {
    return {
      tone: 'info',
      message:
        unanswered > 0
          ? 'No availability checks were added.'
          : 'Every published port already has an availability check.',
      unansweredLine,
    };
  }
  const more =
    result.remaining > 0
      ? ` ${result.remaining} more ${plural(result.remaining, 'port is', 'ports are')} still to add.`
      : '';
  return {
    tone: 'success',
    message: `Added ${created} availability ${plural(created, 'check', 'checks')}.${more}`,
    unansweredLine,
  };
};

export const readDismissedPortChecks = (): Set<string> => {
  try {
    const raw = window.localStorage.getItem(DISMISSED_STORAGE_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    return new Set(Array.isArray(parsed) ? parsed.filter((item) => typeof item === 'string') : []);
  } catch {
    return new Set();
  }
};

/**
 * Remembers ports the user was offered and passed on, keeping only keys that
 * are still on offer so the list cannot grow without bound.
 */
export const writeDismissedPortChecks = (
  dismissed: ReadonlySet<string>,
  stillOffered: DockerPortCheckSuggestion[],
): void => {
  try {
    const live = new Set(stillOffered.map(dockerPortCheckKey));
    const kept = [...dismissed].filter((key) => live.has(key));
    window.localStorage.setItem(DISMISSED_STORAGE_KEY, JSON.stringify(kept));
  } catch {
    // Private windows and blocked storage: the offer simply returns next visit.
  }
};
