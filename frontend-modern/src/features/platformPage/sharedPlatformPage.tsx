import { A } from '@solidjs/router';
import TriangleAlertIcon from 'lucide-solid/icons/triangle-alert';
import { For, Show, createSignal, type Component, type JSX } from 'solid-js';
import { EmptyState } from '@/components/shared/EmptyState';
import { TableCard } from '@/components/shared/TableCard';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import type { Resource } from '@/types/resource';

export type PlatformTabSpec<TabId extends string> = {
  id: TabId;
  label: string;
  path: string;
};

export function PlatformSectionTabs<TabId extends string>(props: {
  tabs: readonly PlatformTabSpec<TabId>[];
  active: TabId;
  ariaLabel: string;
}) {
  return (
    <nav class="flex flex-wrap items-center gap-1 border-b border-border" aria-label={props.ariaLabel}>
      <For each={props.tabs}>
        {(tab) => (
          <A
            href={tab.path}
            class={`inline-flex min-h-10 items-center border-b-2 px-3 text-sm font-medium transition-colors ${
              props.active === tab.id
                ? 'border-blue-500 text-blue-600 dark:text-blue-300'
                : 'border-transparent text-muted hover:border-border hover:text-base-content'
            }`}
            aria-current={props.active === tab.id ? 'page' : undefined}
          >
            {tab.label}
          </A>
        )}
      </For>
    </nav>
  );
}

export function PlatformTableEmptyState(props: {
  icon: JSX.Element;
  title: string;
  description: string;
}) {
  return (
    <TableCard>
      <div class="p-6">
        <EmptyState icon={props.icon} title={props.title} description={props.description} />
      </div>
    </TableCard>
  );
}

export function PlatformErrorState(props: {
  title: string;
  description: string;
  onRefresh: () => void;
}) {
  return (
    <TableCard>
      <div class="p-6">
        <EmptyState
          icon={<TriangleAlertIcon class="h-6 w-6 text-slate-400" />}
          title={props.title}
          description={props.description}
          actions={
            <button
              type="button"
              onClick={props.onRefresh}
              class="inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover"
            >
              Refresh
            </button>
          }
        />
      </div>
    </TableCard>
  );
}

export const PlatformResourceTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  groupingMode?: 'grouped' | 'flat';
}> = (props) => {
  const [expandedResourceId, setExpandedResourceId] = createSignal<string | null>(null);

  return (
    <Show
      when={props.resources.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <UnifiedResourceTable
        resources={props.resources}
        expandedResourceId={expandedResourceId()}
        onExpandedResourceChange={setExpandedResourceId}
        groupingMode={props.groupingMode ?? 'grouped'}
      />
    </Show>
  );
};
