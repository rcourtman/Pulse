import { For, Show } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import type { ProblemResource } from '@/hooks/useDashboardOverview';
import { isInfrastructure, isStorage } from '@/types/resource';
import {
  INFRASTRUCTURE_PATH,
  buildInfrastructurePath,
  buildWorkloadsPath,
  buildStoragePath,
} from '@/routing/resourceLinks';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

interface ProblemResourcesTableProps {
  problems: ProblemResource[];
}

function statusVariant(pr: ProblemResource): 'danger' | 'warning' {
  // Offline or very high metric values → danger; degraded → warning
  return pr.worstValue >= 150 ? 'danger' : 'warning';
}

function resourceLink(pr: ProblemResource): string {
  if (isInfrastructure(pr.resource)) {
    return buildInfrastructurePath({ resource: pr.resource.id });
  }
  if (isStorage(pr.resource)) {
    return buildStoragePath();
  }
  return buildWorkloadsPath({ resource: pr.resource.id });
}

function formatType(type: string): string {
  const labels: Record<string, string> = {
    node: 'Node',
    host: 'Host',
    'docker-host': 'Docker Host',
    'k8s-cluster': 'K8s Cluster',
    'k8s-node': 'K8s Node',
    vm: 'VM',
    'system-container': 'Container',
    'app-container': 'Container',
    pod: 'Pod',
    storage: 'Storage',
    truenas: 'TrueNAS',
  };
  return labels[type] || type;
}

export function ProblemResourcesTable(props: ProblemResourcesTableProps) {
  return (
    <Show when={props.problems.length > 0}>
      <Card padding="none" tone="default">
        <div class="px-4 pt-3.5 pb-2 flex items-center gap-2">
          <AlertTriangleIcon class="w-4 h-4 text-red-500 dark:text-red-400" />
          <h2 class="text-sm font-semibold text-base-content">Problem Resources</h2>
          <span class="text-xs text-muted ml-auto">
            {props.problems.length} resource{props.problems.length !== 1 ? 's' : ''}
          </span>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead class="w-8" />
              <TableHead>Resource</TableHead>
              <TableHead class="hidden sm:table-cell">Type</TableHead>
              <TableHead>Problem</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <For each={props.problems}>
              {(pr) => (
                <TableRow>
                  <TableCell>
                    <StatusDot variant={statusVariant(pr)} size="sm" pulse={pr.worstValue >= 200} />
                  </TableCell>
                  <TableCell>
                    <a
                      href={resourceLink(pr)}
                      class="text-xs font-medium text-base-content hover:underline truncate block max-w-[200px]"
                      title={pr.resource.displayName || pr.resource.name}
                    >
                      {pr.resource.displayName || pr.resource.name}
                    </a>
                  </TableCell>
                  <TableCell class="hidden sm:table-cell">
                    <span class="text-xs text-muted">{formatType(pr.resource.type)}</span>
                  </TableCell>
                  <TableCell>
                    <div class="flex items-center gap-1.5 flex-wrap">
                      <For each={pr.problems}>
                        {(problem) => {
                          const isOffline = problem === 'Offline';
                          const isDegraded = problem === 'Degraded';
                          const badgeClass = isOffline
                            ? 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                            : isDegraded
                              ? 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300'
                              : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300';
                          return (
                            <span
                              class={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium ${badgeClass}`}
                            >
                              {problem}
                            </span>
                          );
                        }}
                      </For>
                    </div>
                  </TableCell>
                </TableRow>
              )}
            </For>
          </TableBody>
        </Table>

        <Show when={props.problems.length >= 8}>
          <div class="px-4 py-2 border-t border-border flex items-center gap-3">
            <a
              href={INFRASTRUCTURE_PATH}
              class="text-[11px] text-blue-600 hover:underline dark:text-blue-400"
            >
              Infrastructure
            </a>
            <a
              href={buildStoragePath()}
              class="text-[11px] text-blue-600 hover:underline dark:text-blue-400"
            >
              Storage
            </a>
          </div>
        </Show>
      </Card>
    </Show>
  );
}

export default ProblemResourcesTable;
