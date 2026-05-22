import { Show, createSignal, type Component, type JSX } from 'solid-js';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';
import { hasTrueNASDetailSections } from '@/components/Infrastructure/resourceDetailDrawerTrueNASModel';
import { hasVMwareDetailSections } from '@/components/Infrastructure/resourceDetailDrawerVmwareModel';
import { TableCell, TableRow } from '@/components/shared/Table';
import type { Resource } from '@/types/resource';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';

type ResourceLike = Pick<Resource, 'id'>;

export type PlatformResourceDetailState = {
  expandedResourceId: () => string | null;
  isExpanded: (resource: ResourceLike) => boolean;
  detailRowId: (resource: ResourceLike) => string;
  toggle: (resource: ResourceLike) => void;
  close: (resource?: ResourceLike) => void;
  handleActivationKey: (
    resource: ResourceLike,
  ) => JSX.EventHandler<HTMLTableRowElement, KeyboardEvent>;
};

export const PLATFORM_RESOURCE_DETAIL_ROW_CLASS =
  'cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60 focus-visible:ring-offset-1 focus-visible:ring-offset-surface';

export const getPlatformResourceDetailRowClass = (expanded: boolean): string =>
  `${PLATFORM_RESOURCE_DETAIL_ROW_CLASS}${expanded ? ' bg-surface-hover' : ''}`;

export function createPlatformResourceDetailState(options: {
  idPrefix: string;
}): PlatformResourceDetailState {
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);

  const isExpanded = (resource: ResourceLike): boolean => expandedResourceId() === resource.id;
  const detailRowId = (resource: ResourceLike): string => `${options.idPrefix}-${resource.id}`;
  const toggle = (resource: ResourceLike) => {
    setExpandedResourceId((current) => (current === resource.id ? null : resource.id));
  };
  const close = (resource?: ResourceLike) => {
    if (!resource || isExpanded(resource)) {
      setExpandedResourceId(null);
    }
  };
  const handleActivationKey =
    (resource: ResourceLike): JSX.EventHandler<HTMLTableRowElement, KeyboardEvent> =>
    (event) => {
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault();
      toggle(resource);
    };

  return {
    expandedResourceId,
    isExpanded,
    detailRowId,
    toggle,
    close,
    handleActivationKey,
  };
}

export function createPlatformResourceLabelResolver(resources: () => readonly Resource[]) {
  return (resourceId: string): string | undefined => {
    const resource = resources().find((candidate) => candidate.id === resourceId);
    return resource ? getPreferredInfrastructureDisplayName(resource) : undefined;
  };
}

export const PlatformResourceDetailTableRow: Component<{
  resource: Resource;
  open: boolean;
  detailRowId: string;
  colSpan: number;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
  initialShowTrueNASDetails?: boolean;
  initialShowVMwareDetails?: boolean;
  onClose?: () => void;
}> = (props) => {
  const initialShowTrueNASDetails = () =>
    props.initialShowTrueNASDetails ?? hasTrueNASDetailSections(props.resource);
  const initialShowVMwareDetails = () =>
    props.initialShowVMwareDetails ?? hasVMwareDetailSections(props.resource);

  return (
    <Show when={props.open}>
      <TableRow
        data-inline-detail-for={props.resource.id}
        data-inline-platform-resource-detail-for={props.resource.id}
      >
        <TableCell
          id={props.detailRowId}
          colspan={props.colSpan}
          class="border-b border-border bg-surface-alt p-0"
        >
          <div class="px-2 py-3 sm:px-4 sm:py-4" onClick={(event) => event.stopPropagation()}>
            <ResourceDetailDrawer
              resource={props.resource}
              presentation="table-row"
              resolveResourceLabel={props.resolveResourceLabel}
              initialShowTrueNASDetails={initialShowTrueNASDetails()}
              initialShowVMwareDetails={initialShowVMwareDetails()}
              onClose={props.onClose}
            />
          </div>
        </TableCell>
      </TableRow>
    </Show>
  );
};
