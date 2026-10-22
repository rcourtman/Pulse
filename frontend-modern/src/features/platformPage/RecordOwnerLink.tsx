import { A } from '@solidjs/router';
import ArrowUpRightIcon from 'lucide-solid/icons/arrow-up-right';
import { Show, type Component } from 'solid-js';
import { getDrawerHeaderActionButtonClass } from '@/components/shared/buttonModel';
import {
  buildResourcePagePath,
  type ResourcePageSubject,
} from '@/features/platformNavigation/resourcePagePath';
import {
  DOCKER_QUERY_PARAMS,
  KUBERNETES_QUERY_PARAMS,
  STANDALONE_QUERY_PARAMS,
  STORAGE_QUERY_PARAMS,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';

// The parameters with which a platform page opens one resource's drawer or
// filters its table down to that resource.
const RESOURCE_FOCUS_PARAMS: ReadonlySet<string> = new Set([
  WORKLOADS_QUERY_PARAMS.resource,
  STORAGE_QUERY_PARAMS.resource,
  STORAGE_QUERY_PARAMS.query,
  DOCKER_QUERY_PARAMS.query,
  KUBERNETES_QUERY_PARAMS.query,
  STANDALONE_QUERY_PARAMS.query,
]);

/**
 * The page that brings the object a record is about into view, or null. A
 * record drawer (an alert, an activity, a backup) links to its owner only
 * where the owner's page can open or filter to it: a link that lands on a
 * whole tab leaves the user hunting for the row it promised.
 */
export function buildRecordOwnerPath(
  subject: ResourcePageSubject | null | undefined,
): string | null {
  if (!subject?.id?.trim()) return null;
  const path = buildResourcePagePath(subject);
  const queryStart = path?.indexOf('?') ?? -1;
  if (!path || queryStart < 0) return null;
  const params = new URLSearchParams(path.slice(queryStart + 1));
  for (const [key, value] of params) {
    if (RESOURCE_FOCUS_PARAMS.has(key) && value.trim()) return path;
  }
  return null;
}

/** A record drawer's header action that opens the object the record is about. */
export const RecordOwnerLink: Component<{
  subject: ResourcePageSubject | null | undefined;
  /** What the link names; defaults to the subject's name. */
  name?: string;
}> = (props) => {
  const href = () => buildRecordOwnerPath(props.subject);
  const label = () => `Open ${props.name?.trim() || props.subject?.name?.trim() || 'resource'}`;
  return (
    <Show when={href()}>
      {(path) => (
        <A
          href={path()}
          class={getDrawerHeaderActionButtonClass('max-w-48 sm:max-w-64')}
          title={label()}
          data-record-owner-link
        >
          <ArrowUpRightIcon class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span class="truncate">{label()}</span>
        </A>
      )}
    </Show>
  );
};
