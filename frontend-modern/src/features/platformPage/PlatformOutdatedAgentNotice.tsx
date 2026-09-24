import { Show, createMemo, createSignal } from 'solid-js';
import { AlertTriangle, ArrowRight } from 'lucide-solid';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import type { OutdatedAgentHost } from './agentVersion';

type PlatformOutdatedAgentNoticeProps = {
  // Resources on this page whose agent is behind the server version.
  hosts: OutdatedAgentHost[];
  // The version users should update to (the server's version). Optional: the
  // notice still reads sensibly without it.
  targetVersion?: string;
  // What the update unlocks, e.g. "images, networks, and storage".
  // Default copy phrases this as missing data; latest-detail copy phrases it as
  // newer agent-contributed detail for hybrid platform pages.
  missingLabel: string;
  copyVariant?: 'missing-data' | 'latest-detail';
  actionHref?: string;
  actionLabel?: string;
  subjectSingular?: string;
  subjectPlural?: string;
};

// Inline, self-explaining notice shown on a platform page when one or more of
// its resources run an agent too old to report part of the page's inventory. It
// is rendered only when there is an actually-outdated resource, so the page
// stays clean in the healthy case. This is the breadcrumb that distinguishes a
// genuinely-empty detail tab from one hidden by a stale agent.
export function PlatformOutdatedAgentNotice(props: PlatformOutdatedAgentNoticeProps) {
  const count = createMemo(() => props.hosts.length);
  const names = createMemo(() => props.hosts.map((host) => host.name).join(', '));
  const [showAllHosts, setShowAllHosts] = createSignal(false);
  const preview = createMemo(() => {
    const firstNames = props.hosts
      .slice(0, 3)
      .map((host) => host.name)
      .join(', ');
    const remaining = count() - 3;
    return remaining > 0 ? `${firstNames}, and ${remaining} more` : firstNames;
  });
  const actionLabel = createMemo(() => props.actionLabel || 'Open Infrastructure settings');
  const subjectSingular = createMemo(() => props.subjectSingular || 'host');
  const subjectPlural = createMemo(() => props.subjectPlural || 'hosts');
  const visible = createMemo(() => count() > 0 && !presentationPolicyIsReadOnly());

  const message = createMemo(() => {
    const target = props.targetVersion ? ` to ${props.targetVersion}` : '';
    const copyVariant = props.copyVariant || 'missing-data';
    if (count() === 1) {
      const host = props.hosts[0];
      if (copyVariant === 'latest-detail') {
        return `${host.name} is running an older Pulse agent (${host.version}). Update it${target} for the latest ${props.missingLabel} on this ${subjectSingular()}.`;
      }
      return `${host.name} is running an older Pulse agent (${host.version}). Update it${target} to see ${props.missingLabel} for this ${subjectSingular()}.`;
    }
    if (copyVariant === 'latest-detail') {
      return `${count()} ${subjectPlural()} are running an older Pulse agent. Update them${target} for the latest ${props.missingLabel}. Affected: ${preview()}.`;
    }
    return `${count()} ${subjectPlural()} are running an older Pulse agent. Update them${target} to see ${props.missingLabel}. Affected: ${preview()}.`;
  });

  return (
    <Show when={visible()}>
      <InlineNotice
        role="status"
        data-testid="platform-outdated-agent-notice"
        tone="warning"
        icon={<AlertTriangle aria-hidden="true" />}
        actionHref={props.actionHref}
        actionLabel={actionLabel()}
        actionIcon={<ArrowRight aria-hidden="true" />}
      >
        <span>{message()}</span>{' '}
        <Show when={count() > 3}>
          <button
            type="button"
            class="ml-1 font-medium underline underline-offset-2 hover:no-underline focus-visible:rounded-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
            aria-expanded={showAllHosts()}
            onClick={() => setShowAllHosts((current) => !current)}
          >
            {showAllHosts() ? 'Hide affected names' : `Show all ${count()} ${subjectPlural()}`}
          </button>
          <Show when={showAllHosts()}>
            <p class="mt-1 break-words">Affected: {names()}.</p>
          </Show>
        </Show>
      </InlineNotice>
    </Show>
  );
}

export default PlatformOutdatedAgentNotice;
