import { Show, createSignal, type Component, type JSX } from 'solid-js';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import type { Resource } from '@/types/resource';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';

type ResourceLike = Pick<Resource, 'id'>;

export type PlatformResourceDetailRowInteractionOptions = {
  expanded: boolean;
  onToggle: () => void;
  class?: string;
};

export type PlatformResourceDetailState = {
  expandedResourceId: () => string | null;
  isExpanded: (resource: ResourceLike) => boolean;
  detailRowId: (resource: ResourceLike) => string;
  open: (resource: ResourceLike) => void;
  toggle: (resource: ResourceLike) => void;
  close: (resource?: ResourceLike) => void;
};

export const PLATFORM_RESOURCE_DETAIL_ROW_CLASS = 'cursor-pointer';

export const getPlatformResourceDetailRowClass = (expanded: boolean): string =>
  `${PLATFORM_RESOURCE_DETAIL_ROW_CLASS}${expanded ? ' bg-surface-hover' : ''}`;

const isInteractiveDetailRowDescendant = (event: MouseEvent): boolean => {
  const target = event.target;
  const currentTarget = event.currentTarget;
  if (
    !(target instanceof Element) ||
    !(currentTarget instanceof Element) ||
    target === currentTarget
  ) {
    return false;
  }
  return Boolean(
    target.closest('a, button, input, select, textarea, [role="button"], [role="link"]'),
  );
};

// Whole-row clicking remains a pointer convenience. The nested native
// disclosure button owns keyboard focus and aria-expanded/aria-controls so a
// static data-table row is not exposed as a second, unnamed control.
export function getPlatformResourceDetailRowInteractionProps(
  options: PlatformResourceDetailRowInteractionOptions,
): JSX.HTMLAttributes<HTMLTableRowElement> {
  return {
    class: `${getPlatformResourceDetailRowClass(options.expanded)} ${options.class ?? ''}`.trim(),
    onClick: (event) => {
      if (!isInteractiveDetailRowDescendant(event)) options.onToggle();
    },
  };
}

export const PlatformResourceDetailToggleButton: Component<{
  expanded: boolean;
  resourceLabel: string;
  controlsId?: string;
  class?: string;
  onToggle: () => void;
}> = (props) => (
  <SummaryRowActionButton
    kind="disclosure"
    expanded={props.expanded}
    subjectLabel={`details for ${props.resourceLabel}`}
    controlsId={props.controlsId}
    class={props.class}
    hideWhenRowTappableOnMobile
    onAction={props.onToggle}
  />
);

export function createPlatformResourceDetailState(options: {
  idPrefix: string;
}): PlatformResourceDetailState {
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);

  const isExpanded = (resource: ResourceLike): boolean => expandedResourceId() === resource.id;
  const detailRowId = (resource: ResourceLike): string => `${options.idPrefix}-${resource.id}`;
  const open = (resource: ResourceLike) => {
    setExpandedResourceId(resource.id);
  };
  const toggle = (resource: ResourceLike) => {
    setExpandedResourceId((current) => (current === resource.id ? null : resource.id));
  };
  const close = (resource?: ResourceLike) => {
    if (!resource || isExpanded(resource)) {
      setExpandedResourceId(null);
    }
  };
  return {
    expandedResourceId,
    isExpanded,
    detailRowId,
    open,
    toggle,
    close,
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
  initialShowAccessContext?: boolean;
  initialShowHostDetails?: boolean;
  initialShowTrueNASDetails?: boolean;
  onResourceActionSettled?: () => void | Promise<void>;
  onClose?: () => void;
}> = (props) => {
  return (
    <Show when={props.open}>
      <InlineDetailTableRow
        cellId={props.detailRowId}
        colspan={props.colSpan}
        data-inline-detail-for={props.resource.id}
        data-inline-platform-resource-detail-for={props.resource.id}
      >
        <ResourceDetailDrawer
          resource={props.resource}
          presentation="table-row"
          resolveResourceLabel={props.resolveResourceLabel}
          initialShowAccessContext={props.initialShowAccessContext}
          initialShowHostDetails={props.initialShowHostDetails}
          initialShowTrueNASDetails={props.initialShowTrueNASDetails}
          onResourceActionSettled={props.onResourceActionSettled}
          onClose={props.onClose}
        />
      </InlineDetailTableRow>
    </Show>
  );
};
