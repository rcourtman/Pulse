import { Show, createMemo } from 'solid-js';
import { SectionHeader } from '@/components/shared/SectionHeader';
import { EmptyState } from '@/components/shared/EmptyState';
import { Card } from '@/components/shared/Card';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';
import ServerIcon from 'lucide-solid/icons/server';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

export function Infrastructure() {
  const { resources, loading, error, refetch } = useUnifiedResources();
  const hasResources = createMemo(() => resources().length > 0);

  return (
    <div class="space-y-4 px-4">
      <SectionHeader
        title="Infrastructure"
        description="Unified host inventory across monitored platforms."
        size="lg"
      />

      <Show when={!loading()} fallback={
        <Card class="p-6">
          <div class="text-sm text-gray-600 dark:text-gray-300">Loading infrastructure resources...</div>
        </Card>
      }>
        <Show
          when={!error()}
          fallback={
            <Card class="p-6">
              <EmptyState
                icon={<ServerIcon class="w-6 h-6 text-gray-400" />}
                title="Unable to load infrastructure"
                description="We couldn’t fetch unified resources. Check connectivity or retry."
                actions={
                  <button
                    type="button"
                    onClick={() => refetch()}
                    class="inline-flex items-center gap-2 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
                  >
                    <RefreshCwIcon class="h-3.5 w-3.5" />
                    Retry
                  </button>
                }
              />
            </Card>
          }
        >
          <Show
            when={hasResources()}
            fallback={
              <Card class="p-6">
                <EmptyState
                  icon={<ServerIcon class="w-6 h-6 text-gray-400" />}
                  title="No infrastructure resources yet"
                  description="Once hosts are reporting, they will appear here."
                />
              </Card>
            }
          >
            <UnifiedResourceTable resources={resources()} />
          </Show>
        </Show>
      </Show>
    </div>
  );
}

export default Infrastructure;
