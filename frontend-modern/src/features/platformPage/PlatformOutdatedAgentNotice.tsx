import { Show, createMemo, createSignal } from 'solid-js';
import { ArrowRight, Info } from 'lucide-solid';
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
//
// It is maintenance, not an incident, so it is one quiet line in the info tone
// rather than a multi-line warning above every tab: the affected names sit
// behind a toggle and the update action stays one click away.
export function PlatformOutdatedAgentNotice(props: PlatformOutdatedAgentNoticeProps) {
  const count = createMemo(() => props.hosts.length);
  const names = createMemo(() => props.hosts.map((host) => host.name).join(', '));
  const [showAllHosts, setShowAllHosts] = createSignal(false);
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
        return `${host.name} runs an older Pulse agent (${host.version}). Update it${target} for the latest ${props.missingLabel}.`;
      }
      return `${host.name} runs an older Pulse agent (${host.version}), so ${props.missingLabel} for this ${subjectSingular()} may be missing.`;
    }
    if (copyVariant === 'latest-detail') {
      return `${count()} ${subjectPlural()} run an older Pulse agent. Update them${target} for the latest ${props.missingLabel}.`;
    }
    return `${count()} ${subjectPlural()} run an older Pulse agent, so ${props.missingLabel} may be missing.`;
  });

  return (
    <Show when={visible()}>
      <InlineNotice
        role="status"
        data-testid="platform-outdated-agent-notice"
        tone="info"
        icon={<Info aria-hidden="true" />}
      >
        <span>{message()}</span>
        <Show when={count() > 1}>
          {' '}
          <button
            type="button"
            class="font-medium underline underline-offset-2 hover:no-underline focus-visible:rounded-xs focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2"
            aria-expanded={showAllHosts()}
            onClick={() => setShowAllHosts((current) => !current)}
          >
            {showAllHosts() ? 'Hide names' : `Which ${subjectPlural()}?`}
          </button>
        </Show>
        <Show when={props.actionHref}>
          {(href) => (
            <>
              {' '}
              <a
                href={href()}
                class="inline-flex items-center gap-1 whitespace-nowrap font-semibold underline-offset-2 hover:underline"
              >
                {actionLabel()}
                <ArrowRight aria-hidden="true" class="h-3.5 w-3.5" />
              </a>
            </>
          )}
        </Show>
        <Show when={count() > 1 && showAllHosts()}>
          <p class="mt-1 wrap-break-word">Affected: {names()}.</p>
        </Show>
      </InlineNotice>
    </Show>
  );
}

export default PlatformOutdatedAgentNotice;
