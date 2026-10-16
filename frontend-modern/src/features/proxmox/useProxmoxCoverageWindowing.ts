import { createEffect, createMemo, createSignal, type Accessor } from 'solid-js';

import { usePlatformWindowedItems } from '@/features/platformPage/usePlatformWindowedItems';
import type { WorkloadCoverageRow } from './proxmoxBackupRecoveryModel';

// Both wrapped identity and expanded evidence belong to a workload, not its
// sorted index. Unseen rows use estimates until their actual block is mounted.
export function useProxmoxCoverageWindowing(
  rows: Accessor<readonly WorkloadCoverageRow[]>,
  expanded: Accessor<ReadonlySet<string>>,
  measurementKey: Accessor<unknown>,
  layoutKey: Accessor<unknown>,
) {
  const [summaryHeights, setSummaryHeights] = createSignal<ReadonlyMap<string, number>>(new Map());
  const [detailHeights, setDetailHeights] = createSignal<ReadonlyMap<string, number>>(new Map());
  // A width change invalidates off-screen wrapping measurements too. Keeping
  // old phone heights after widening (or vice versa) creates false runway.
  createEffect(() => {
    layoutKey();
    setSummaryHeights(new Map());
    setDetailHeights(new Map());
  });
  createEffect(() => {
    const keys = new Set(rows().map((row) => row.key));
    const retain = (current: ReadonlyMap<string, number>) => {
      if ([...current.keys()].every((key) => keys.has(key))) return current;
      return new Map([...current].filter(([key]) => keys.has(key)));
    };
    setSummaryHeights(retain);
    setDetailHeights(retain);
  });
  const offsets = createMemo(() => {
    const heights = detailHeights();
    const summaries = summaryHeights();
    const open = expanded();
    const estimate = summaries.size
      ? [...summaries.values()].reduce((sum, height) => sum + height, 0) / summaries.size
      : 32;
    const result = [0];
    for (const row of rows()) {
      const height =
        (summaries.get(row.key) ?? estimate) +
        (open.has(row.key) ? (heights.get(row.key) ?? 320) : 0);
      result.push(result[result.length - 1]! + height);
    }
    return result;
  });
  const measure = (root: HTMLElement) => {
    const body = root.querySelector('tbody');
    if (!body) return;
    const summaries = Array.from(body.children).filter(
      (row): row is HTMLElement =>
        row instanceof HTMLElement && row.dataset.proxmoxBackupRow === 'coverage',
    );
    setSummaryHeights((current) => {
      let next: Map<string, number> | undefined;
      for (const summary of summaries) {
        const key = summary.dataset.proxmoxCoverageKey;
        const measured = summary.getBoundingClientRect().height;
        if (!key || !Number.isFinite(measured) || measured <= 0 || current.get(key) === measured)
          continue;
        next ??= new Map(current);
        next.set(key, measured);
      }
      return next ?? current;
    });
    setDetailHeights((current) => {
      let next: Map<string, number> | undefined;
      for (const summary of summaries) {
        const key = summary.dataset.proxmoxCoverageKey;
        const detail = summary.nextElementSibling;
        if (!key || detail?.getAttribute('data-inline-detail-for') !== key) continue;
        const measured = detail.getBoundingClientRect().height;
        if (!Number.isFinite(measured) || measured <= 0 || current.get(key) === measured) continue;
        next ??= new Map(current);
        next.set(key, measured);
      }
      return next ?? current;
    });
  };
  const state = usePlatformWindowedItems({
    items: rows,
    estimatedItemHeight: 32,
    itemGeometry: { offsets, measure },
    measurementKey,
  });
  return { ...state, setRootRef: state.setAnchorRef };
}
