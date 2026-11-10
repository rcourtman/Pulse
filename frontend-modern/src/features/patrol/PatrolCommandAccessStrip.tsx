import { createMemo, For, Show } from 'solid-js';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import { ButtonLink } from '@/components/shared/Button';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { buildInfrastructureCommandAccessPath } from '@/components/Settings/infrastructureWorkspaceModel';
import { useResources } from '@/hooks/useResources';
import { getUnifiedAgentCommandAccessOptionLabel } from '@/utils/unifiedAgentInventoryPresentation';
import type { PatrolAutonomyLevel } from '@/api/patrol';
import {
  buildPatrolCommandAccessSummary,
  buildPatrolGuestActionAccessSummary,
  PATROL_COMMAND_ACCESS_ACTION_LABEL,
  patrolModeNeedsCommandAccess,
  type PatrolCommandAccessHost,
} from './patrolCommandAccessModel';

// Enough names to recognise a small estate at a glance. The rest sit behind the
// count, one click away, so a default install with every host listed stays one row.
const PREVIEW_HOSTS = 3;

const summaryClass =
  'flex min-h-11 cursor-pointer list-none flex-wrap items-center gap-x-2 gap-y-1 text-xs focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 sm:min-h-0 [&::-webkit-details-marker]:hidden';

function NameBadge(props: { name: string }) {
  return (
    <MetadataBadge tone="neutral" size="xs" shape="rounded" appearance="outline">
      {props.name}
    </MetadataBadge>
  );
}

/** The first names, shown while the row is closed, and a count of the rest. */
function NamePreview(props: { names: PatrolCommandAccessHost[] }) {
  const preview = createMemo(() => props.names.slice(0, PREVIEW_HOSTS));
  const hiddenCount = createMemo(() => props.names.length - preview().length);
  return (
    <span class="flex min-w-0 flex-wrap items-center gap-1 group-open:hidden">
      <For each={preview()}>{(entry) => <NameBadge name={entry.name} />}</For>
      <Show when={hiddenCount() > 0}>
        <span class="text-muted">+{hiddenCount()} more</span>
      </Show>
    </span>
  );
}

function NameList(props: { names: PatrolCommandAccessHost[]; label: string }) {
  return (
    <ul class="flex flex-wrap gap-1" aria-label={props.label}>
      <For each={props.names}>
        {(entry) => (
          <li>
            <NameBadge name={entry.name} />
          </li>
        )}
      </For>
    </ul>
  );
}

/**
 * Patrol reads logs and runs fixes through a host's Pulse agent, and agents
 * install with commands off. The first row names the hosts Patrol can only
 * watch and carries the one step that changes it.
 *
 * Starting, stopping and restarting a Proxmox guest is separate: it runs only
 * through an action runner on the node, and ticking the install option does
 * not cover it. The second row names the nodes where Pulse cannot do that. It
 * has no button, because no single step connects a runner.
 *
 * Both rows are absent in Watch only, which never reads a host or proposes a
 * fix, and each is absent when it has nothing to name.
 */
export function PatrolCommandAccessStrip(props: {
  autonomyLevel: PatrolAutonomyLevel;
  readOnly?: boolean;
}) {
  const { resources } = useResources();
  const summary = createMemo(() => buildPatrolCommandAccessSummary(resources()));
  const guestSummary = createMemo(() => buildPatrolGuestActionAccessSummary(resources()));
  const needsAccess = createMemo(() => patrolModeNeedsCommandAccess(props.autonomyLevel));
  const visible = createMemo(() => needsAccess() && summary().commandsOff.length > 0);
  const guestVisible = createMemo(() => needsAccess() && guestSummary().noRunner.length > 0);

  return (
    <>
      <Show when={visible()}>
        <div
          class="flex flex-col gap-2 border-t border-border-subtle px-1 py-2 sm:flex-row sm:items-start sm:px-2"
          data-testid="patrol-command-access"
        >
          <details class="group min-w-0 flex-1">
            <summary class={summaryClass}>
              <ChevronRightIcon
                class="h-3.5 w-3.5 shrink-0 text-muted transition-transform group-open:rotate-90"
                aria-hidden="true"
              />
              <span class="font-medium text-base-content">Can't read logs or run fixes</span>
              <MetadataBadge tone="warning" size="xs" shape="pill">
                {summary().commandsOff.length} of {summary().total}{' '}
                {summary().total === 1 ? 'host' : 'hosts'}
              </MetadataBadge>
              <NamePreview names={summary().commandsOff} />
            </summary>
            <div class="space-y-2 pb-1 pt-2">
              <NameList names={summary().commandsOff} label="Hosts with commands off" />
              <p class="max-w-3xl text-xs leading-5 text-muted">
                Commands are off on the Pulse agent on these hosts, so Patrol can only watch them.
                To allow more, run the install command on a host with "
                {getUnifiedAgentCommandAccessOptionLabel()}" ticked.
              </p>
            </div>
          </details>
          <Show when={!props.readOnly}>
            <ButtonLink
              href={buildInfrastructureCommandAccessPath()}
              variant="outline"
              size="xs"
              class="shrink-0 self-start"
            >
              {PATROL_COMMAND_ACCESS_ACTION_LABEL}
            </ButtonLink>
          </Show>
        </div>
      </Show>
      <Show when={guestVisible()}>
        <div
          class="border-t border-border-subtle px-1 py-2 sm:px-2"
          data-testid="patrol-guest-action-access"
        >
          <details class="group min-w-0">
            <summary class={summaryClass}>
              <ChevronRightIcon
                class="h-3.5 w-3.5 shrink-0 text-muted transition-transform group-open:rotate-90"
                aria-hidden="true"
              />
              <span class="font-medium text-base-content">Can't start or stop guests</span>
              <MetadataBadge tone="neutral" size="xs" shape="pill">
                {guestSummary().noRunner.length} of {guestSummary().total} Proxmox{' '}
                {guestSummary().total === 1 ? 'node' : 'nodes'}
              </MetadataBadge>
              <NamePreview names={guestSummary().noRunner} />
            </summary>
            <div class="space-y-2 pb-1 pt-2">
              <NameList
                names={guestSummary().noRunner}
                label="Proxmox nodes with no action runner"
              />
              <p class="max-w-3xl text-xs leading-5 text-muted">
                No Pulse action runner is connected on these nodes, so Pulse cannot start, stop or
                restart their guests. Patrol tells you when a guest needs it, and you do it in
                Proxmox. "{getUnifiedAgentCommandAccessOptionLabel()}" does not cover guest actions.
              </p>
            </div>
          </details>
        </div>
      </Show>
    </>
  );
}
