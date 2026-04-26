import { createMemo, For, Show } from 'solid-js';
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
  buildInfrastructureResourceHref,
  buildWorkloadsPath,
  buildStoragePath,
} from '@/routing/resourceLinks';
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
import { getTypeColumnLabel } from '@/utils/typeColumnPresentation';
import { getSimpleStatusIndicator, getStatusIndicatorBadgeToneClasses } from '@/utils/status';
import { getProblemResourceStatusVariant } from '@/utils/problemResourcePresentation';
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';

interface ProblemResourcesTableProps {
  problems: ProblemResource[];
}

interface ProblemResourceGroup {
  representative: ProblemResource;
  resources: ProblemResource[];
  displayName: string;
  typeLabel: string;
  worstValue: number;
  memberDisplayNames: string[];
}

function resourceLink(pr: ProblemResource): string {
  if (isInfrastructure(pr.resource)) {
    return buildInfrastructureResourceHref(pr.resource.id) ?? INFRASTRUCTURE_PATH;
  }
  if (isStorage(pr.resource)) {
    return buildStoragePath();
  }
  return buildWorkloadsPath({ resource: pr.resource.id });
}

function normalizeGroupValue(value: string): string {
  return value.trim().toLowerCase();
}

function problemResourceDisplayName(pr: ProblemResource): string {
  return getPreferredResourceDisplayName(pr.resource) || pr.resource.id;
}

function problemGroupKey(pr: ProblemResource): string {
  const displayName = problemResourceDisplayName(pr);
  const problems = pr.problems.map(normalizeGroupValue).sort().join('|');
  return [normalizeGroupValue(displayName), pr.resource.type, problems].join('::');
}

function problemResourceGroups(problems: ProblemResource[]): ProblemResourceGroup[] {
  const groups = new Map<string, ProblemResourceGroup>();

  for (const problem of problems) {
    const key = problemGroupKey(problem);
    const existing = groups.get(key);
    if (existing) {
      existing.resources.push(problem);
      existing.worstValue = Math.max(existing.worstValue, problem.worstValue);
      const memberName = problemResourceDisplayName(problem);
      if (!existing.memberDisplayNames.includes(memberName)) {
        existing.memberDisplayNames.push(memberName);
      }
      continue;
    }

    const displayName = problemResourceDisplayName(problem);
    groups.set(key, {
      representative: problem,
      resources: [problem],
      displayName,
      typeLabel: getResourceTypeLabel(problem.resource.type) || problem.resource.type,
      worstValue: problem.worstValue,
      memberDisplayNames: [displayName || problem.resource.id],
    });
  }

  return Array.from(groups.values());
}

function groupedResourceLink(group: ProblemResourceGroup): string {
  if (group.resources.length === 1) {
    return resourceLink(group.representative);
  }
  if (group.resources.every((problem) => isStorage(problem.resource))) {
    return buildStoragePath();
  }
  if (group.resources.every((problem) => isInfrastructure(problem.resource))) {
    return INFRASTRUCTURE_PATH;
  }
  return buildWorkloadsPath({ type: group.representative.resource.type });
}

function pluralizeTypeLabel(count: number, label: string): string {
  const normalized = (label || 'resource').trim().toLowerCase();
  if (count === 1) return normalized;
  if (normalized === 'storage') return 'storage resources';
  if (normalized.endsWith('s')) return normalized;
  if (normalized.endsWith('y')) return `${normalized.slice(0, -1)}ies`;
  return `${normalized}s`;
}

export function ProblemResourcesTable(props: ProblemResourcesTableProps) {
  const groupedProblems = createMemo(() => problemResourceGroups(props.problems));

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
            <For each={groupedProblems()}>
              {(group) => (
                <TableRow>
                  <TableCell>
                    <StatusDot
                      variant={getProblemResourceStatusVariant(group.worstValue)}
                      size="sm"
                      pulse={group.worstValue >= 200}
                    />
                  </TableCell>
                  <TableCell>
                    <Show
                      when={group.resources.length > 1}
                      fallback={
                        <a
                          href={groupedResourceLink(group)}
                          class="text-xs font-medium text-base-content hover:underline truncate block max-w-[200px]"
                          title={group.displayName}
                        >
                          {group.displayName}
                        </a>
                      }
                    >
                      <a
                        href={groupedResourceLink(group)}
                        class="text-xs font-medium text-base-content hover:underline truncate block max-w-[200px]"
                        title={group.memberDisplayNames.join(', ')}
                      >
                        {group.resources.length}{' '}
                        {pluralizeTypeLabel(group.resources.length, group.typeLabel)}
                      </a>
                      <span
                        class="mt-0.5 block text-[10px] text-muted truncate max-w-[220px]"
                        title={group.memberDisplayNames.join(', ')}
                      >
                        {group.memberDisplayNames.join(', ')}
                      </span>
                    </Show>
                  </TableCell>
                  <TableCell class="hidden sm:table-cell">
                    <span class="text-xs text-muted">{group.typeLabel}</span>
                  </TableCell>
                  <TableCell>
                    <div class="flex items-center gap-1.5 flex-wrap">
                      <For each={group.representative.problems}>
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
