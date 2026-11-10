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
  PATROL_COMMAND_ACCESS_ACTION_LABEL,
  patrolModeNeedsCommandAccess,
} from './patrolCommandAccessModel';

// Enough names to recognise a small estate at a glance. The rest sit behind the
// count, one click away, so a default install with every host listed stays one row.
const PREVIEW_HOSTS = 3;

/**
 * Patrol reads logs and runs fixes through a host's Pulse agent, and agents
 * install with commands off. This row names the hosts Patrol can only watch
 * and carries the one step that changes it. It is absent in Watch only, which
 * never reads a host, and when every host is ready.
 */
export function PatrolCommandAccessStrip(props: {
  autonomyLevel: PatrolAutonomyLevel;
  readOnly?: boolean;
}) {
  const { resources } = useResources();
  const summary = createMemo(() => buildPatrolCommandAccessSummary(resources()));
  const visible = createMemo(
    () => patrolModeNeedsCommandAccess(props.autonomyLevel) && summary().commandsOff.length > 0,
  );
  const preview = createMemo(() => summary().commandsOff.slice(0, PREVIEW_HOSTS));
  const hiddenCount = createMemo(() => summary().commandsOff.length - preview().length);

  return (
    <Show when={visible()}>
      <div
        class="flex flex-col gap-2 border-t border-border-subtle px-1 py-2 sm:flex-row sm:items-start sm:px-2"
        data-testid="patrol-command-access"
      >
        <details class="group min-w-0 flex-1">
          <summary class="flex min-h-11 cursor-pointer list-none flex-wrap items-center gap-x-2 gap-y-1 text-xs focus:outline-hidden focus-visible:ring-2 focus-visible:ring-blue-500 sm:min-h-0 [&::-webkit-details-marker]:hidden">
            <ChevronRightIcon
              class="h-3.5 w-3.5 shrink-0 text-muted transition-transform group-open:rotate-90"
              aria-hidden="true"
            />
            <span class="font-medium text-base-content">Can't read logs or run fixes</span>
            <MetadataBadge tone="warning" size="xs" shape="pill">
              {summary().commandsOff.length} of {summary().total}{' '}
              {summary().total === 1 ? 'host' : 'hosts'}
            </MetadataBadge>
            <span class="flex min-w-0 flex-wrap items-center gap-1 group-open:hidden">
              <For each={preview()}>
                {(host) => (
                  <MetadataBadge tone="neutral" size="xs" shape="rounded" appearance="outline">
                    {host.name}
                  </MetadataBadge>
                )}
              </For>
              <Show when={hiddenCount() > 0}>
                <span class="text-muted">+{hiddenCount()} more</span>
              </Show>
            </span>
          </summary>
          <div class="space-y-2 pb-1 pt-2">
            <ul class="flex flex-wrap gap-1" aria-label="Hosts with commands off">
              <For each={summary().commandsOff}>
                {(host) => (
                  <li>
                    <MetadataBadge tone="neutral" size="xs" shape="rounded" appearance="outline">
                      {host.name}
                    </MetadataBadge>
                  </li>
                )}
              </For>
            </ul>
            <p class="max-w-3xl text-xs leading-5 text-muted">
              Commands are off on the Pulse agent on these hosts, so Patrol can only watch them. To
              allow more, run the install command on a host with "
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
  );
}
