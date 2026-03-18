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
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';
import { getSimpleStatusIndicator, getStatusIndicatorBadgeToneClasses } from '@/utils/status';
import { getProblemResourceStatusVariant } from '@/utils/problemResourcePresentation';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

interface ProblemResourcesTableProps {
  problems: ProblemResource[];
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

export function ProblemResourcesTable(props: ProblemResourcesTableProps) {
  return (
    <Show when={props.problems.length > 0}>
      <Card padding="none" tone="default" class="overflow-hidden">
        <div class="px-4 py-3 flex items-center gap-2 bg-red-50/40 dark:bg-red-950/20 border-b border-red-100 dark:border-red-900/30">
          <div class="flex items-center justify-center w-6 h-6 rounded-full bg-red-100 dark:bg-red-900/50">
            <AlertTriangleIcon
              class="w-3.5 h-3.5 text-red-600 dark:text-red-400"
              aria-hidden="true"
            />
          </div>
          <h2 class="text-sm font-semibold text-base-content">Problem Resources</h2>
          <span class="ml-auto text-[10px] font-medium text-red-700 dark:text-red-300 bg-red-100 dark:bg-red-900/50 px-1.5 py-0.5 rounded-full">
            {props.problems.length}
          </span>
        </div>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHead class="w-8" />
              <TableHead>Resource</TableHead>
              <TableHead class="hidden sm:table-cell">{getTypeColumnLabel()}</TableHead>
              <TableHead>Problem</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <For each={props.problems}>
              {(pr) => (
                <TableRow>
                  <TableCell>
                    <StatusDot
                      variant={getProblemResourceStatusVariant(pr.worstValue)}
                      size="sm"
                      pulse={pr.worstValue >= 200}
                    />
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
                    <span class="text-xs text-muted">
                      {getResourceTypeLabel(pr.resource.type) || pr.resource.type}
                    </span>
                  </TableCell>
                  <TableCell>
                    <div class="flex items-center gap-1.5 flex-wrap">
                      <For each={pr.problems}>
                        {(problem) => {
                          const indicator = getSimpleStatusIndicator(problem);
                          return (
                            <span
                              class={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium ${getStatusIndicatorBadgeToneClasses(indicator.variant)}`}
                            >
                              {indicator.label}
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
              href={buildWorkloadsPath()}
              class="text-[11px] text-blue-600 hover:underline dark:text-blue-400"
            >
              Workloads
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
