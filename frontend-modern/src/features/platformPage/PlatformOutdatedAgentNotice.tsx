import { Show, createMemo } from 'solid-js';
import { AlertTriangle } from 'lucide-solid';
import type { OutdatedAgentHost } from './agentVersion';

type PlatformOutdatedAgentNoticeProps = {
  // Hosts on this page whose agent is behind the server version.
  hosts: OutdatedAgentHost[];
  // The version users should update to (the server's version). Optional: the
  // notice still reads sensibly without it.
  targetVersion?: string;
  // What the stale agents cannot report, e.g. "images, networks, and storage".
  // Phrased to slot into "to see {missingLabel} for ...".
  missingLabel: string;
};

// Inline, self-explaining notice shown on a platform page when one or more of
// its hosts run an agent too old to report part of the page's inventory. It is
// rendered only when there is an actually-outdated host, so the page stays
// clean in the healthy case. This is the breadcrumb that distinguishes a
// genuinely-empty detail tab from one hidden by a stale agent.
export function PlatformOutdatedAgentNotice(props: PlatformOutdatedAgentNoticeProps) {
  const count = createMemo(() => props.hosts.length);
  const names = createMemo(() => props.hosts.map((host) => host.name).join(', '));

  const message = createMemo(() => {
    const target = props.targetVersion ? ` to ${props.targetVersion}` : '';
    if (count() === 1) {
      const host = props.hosts[0];
      return `${host.name} is running an older Pulse agent (${host.version}). Update it${target} to see ${props.missingLabel} for this host.`;
    }
    return `${count()} hosts are running an older Pulse agent. Update them${target} to see ${props.missingLabel}. Affected: ${names()}.`;
  });

  return (
    <Show when={count() > 0}>
      <div
        role="status"
        data-testid="platform-outdated-agent-notice"
        class="flex items-start gap-2 rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-200"
      >
        <AlertTriangle class="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <span>{message()}</span>
      </div>
    </Show>
  );
}

export default PlatformOutdatedAgentNotice;
