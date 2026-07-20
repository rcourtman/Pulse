import { For, mergeProps, splitProps, type JSX } from 'solid-js';

type SettingsSkeletonPadding = 'none' | 'panel';
type SettingsSkeletonGap = 'sm' | 'md';
type SettingsSkeletonRadius = 'none' | 'md' | 'full';
type SettingsSkeletonSurface = 'default' | 'muted';
type SettingsSkeletonColumns = 'two' | 'four';

type SettingsSkeletonBlockProps = Omit<JSX.HTMLAttributes<HTMLDivElement>, 'class'> & {
  class?: string;
  radius?: SettingsSkeletonRadius;
  surface?: SettingsSkeletonSurface;
};

type SettingsLoadingSkeletonProps = Omit<JSX.HTMLAttributes<HTMLDivElement>, 'class'> & {
  children: JSX.Element;
  class?: string;
  gap?: SettingsSkeletonGap;
  label?: string;
  padding?: SettingsSkeletonPadding;
};

type SettingsSkeletonMetricGridProps = {
  columns?: SettingsSkeletonColumns;
  count?: number;
  labelWidth?: string;
  valueWidth?: string;
};

type SettingsSkeletonCardProps = Omit<JSX.HTMLAttributes<HTMLDivElement>, 'class'> & {
  children: JSX.Element;
  class?: string;
};

type SettingsSkeletonProgressCardProps = {
  rows?: number;
  titleWidth?: string;
};

type SettingsSkeletonTableCell = {
  class: string;
  radius?: SettingsSkeletonRadius;
};

type SettingsSkeletonTableProps = {
  cells: SettingsSkeletonTableCell[];
  headerClass?: string;
  rowLayoutClass?: string;
  rows?: number;
  titleWidth?: string;
};

const paddingClassByPadding: Record<SettingsSkeletonPadding, string> = {
  none: '',
  panel: 'p-4 sm:p-6',
};

const gapClassByGap: Record<SettingsSkeletonGap, string> = {
  sm: 'space-y-3',
  md: 'space-y-5',
};

const radiusClassByRadius: Record<SettingsSkeletonRadius, string> = {
  none: '',
  md: 'rounded',
  full: 'rounded-full',
};

const surfaceClassBySurface: Record<SettingsSkeletonSurface, string> = {
  default: 'bg-surface-hover',
  muted: 'bg-surface-alt',
};

const metricGridClassByColumns: Record<SettingsSkeletonColumns, string> = {
  two: 'grid gap-3 sm:grid-cols-2',
  four: 'grid gap-3 sm:grid-cols-2 lg:grid-cols-4',
};

const range = (count: number) => Array.from({ length: count }, (_, index) => index);

export function SettingsLoadingSkeleton(props: SettingsLoadingSkeletonProps) {
  const merged = mergeProps(
    { gap: 'md' as SettingsSkeletonGap, label: 'Loading settings', padding: 'none' as const },
    props,
  );
  const [local, rest] = splitProps(merged, ['children', 'class', 'gap', 'label', 'padding']);

  return (
    <div
      {...rest}
      class={`${gapClassByGap[local.gap]} ${paddingClassByPadding[local.padding]} ${
        local.class ?? ''
      }`.trim()}
      role="status"
      aria-label={local.label}
    >
      {local.children}
    </div>
  );
}

export function SettingsSkeletonBlock(props: SettingsSkeletonBlockProps) {
  const merged = mergeProps(
    { radius: 'md' as SettingsSkeletonRadius, surface: 'default' as SettingsSkeletonSurface },
    props,
  );
  const [local, rest] = splitProps(merged, ['class', 'radius', 'surface']);

  return (
    <div
      {...rest}
      class={`animate-pulse ${radiusClassByRadius[local.radius]} ${
        surfaceClassBySurface[local.surface]
      } ${local.class ?? ''}`.trim()}
      aria-hidden="true"
    />
  );
}

export function SettingsSkeletonCard(props: SettingsSkeletonCardProps) {
  const [local, rest] = splitProps(props, ['children', 'class']);

  return (
    <div
      {...rest}
      class={`rounded-md border border-border p-3 space-y-2 ${local.class ?? ''}`.trim()}
    >
      {local.children}
    </div>
  );
}

export function SettingsSkeletonMetricGrid(props: SettingsSkeletonMetricGridProps) {
  const merged = mergeProps(
    {
      columns: 'four' as SettingsSkeletonColumns,
      count: 4,
      labelWidth: 'w-20',
      valueWidth: 'w-28',
    },
    props,
  );

  return (
    <div class={metricGridClassByColumns[merged.columns]}>
      <For each={range(merged.count)}>
        {() => (
          <SettingsSkeletonCard>
            <SettingsSkeletonBlock class={`h-3 ${merged.labelWidth}`} />
            <SettingsSkeletonBlock class={`h-5 ${merged.valueWidth}`} />
          </SettingsSkeletonCard>
        )}
      </For>
    </div>
  );
}

export function SettingsSkeletonProgressCard(props: SettingsSkeletonProgressCardProps) {
  const merged = mergeProps({ rows: 2, titleWidth: 'w-36' }, props);

  return (
    <div class="space-y-3 rounded-md border border-border p-4">
      <SettingsSkeletonBlock class={`h-4 ${merged.titleWidth}`} />
      <For each={range(merged.rows)}>
        {() => (
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <SettingsSkeletonBlock class="h-3 w-14" />
              <SettingsSkeletonBlock class="h-3 w-20" />
            </div>
            <SettingsSkeletonBlock class="h-2 w-full" />
          </div>
        )}
      </For>
    </div>
  );
}

export function SettingsSkeletonTable(props: SettingsSkeletonTableProps) {
  const merged = mergeProps(
    {
      headerClass: 'h-10 w-full',
      rowLayoutClass: 'flex items-center gap-3',
      rows: 3,
    },
    props,
  );

  const table = (
    <div class="overflow-hidden rounded-md border border-border">
      <SettingsSkeletonBlock class={merged.headerClass} radius="none" surface="muted" />
      <For each={range(merged.rows)}>
        {() => (
          <div class="border-t border-border-subtle px-3 py-3">
            <div class={merged.rowLayoutClass}>
              <For each={merged.cells}>
                {(cell) => (
                  <SettingsSkeletonBlock class={cell.class} radius={cell.radius ?? 'md'} />
                )}
              </For>
            </div>
          </div>
        )}
      </For>
    </div>
  );

  if (!merged.titleWidth) {
    return table;
  }

  return (
    <div class="space-y-2">
      <SettingsSkeletonBlock class={`h-4 ${merged.titleWidth}`} />
      {table}
    </div>
  );
}
