import { describe, expect, it } from 'vitest';
import type { PatrolRunRecord } from '@/api/patrol';
import {
  getPatrolRunKindLabel,
  getPatrolRunCoverageSummary,
  getPatrolRunResourcesHeading,
  getPatrolRunStatusPresentation,
  isPatrolRunHealthy,
  getRunHistoryLoadingState,
  getToolCallsLoadingState,
  getToolCallsUnavailableState,
  getToolCallResultBadgeClass,
  getToolCallResultTextClass,
} from '@/utils/patrolRunPresentation';

describe('patrolRunPresentation', () => {
  const scopedCoverageRun: Pick<
    PatrolRunRecord,
    'resources_checked' | 'scope_resource_ids' | 'effective_scope_resource_ids'
  > = {
    resources_checked: 1,
    scope_resource_ids: ['seed-resource'],
    effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
  };

  const fullCoverageRun: Pick<
    PatrolRunRecord,
    'resources_checked' | 'scope_resource_ids' | 'effective_scope_resource_ids'
  > = {
    resources_checked: 58,
    scope_resource_ids: [],
    effective_scope_resource_ids: [],
  };

  it('maps critical and error runs to danger styling', () => {
    expect(getPatrolRunStatusPresentation('critical').badgeClass).toContain('red-100');
    expect(getPatrolRunStatusPresentation('error').badgeClass).toContain('red-100');
  });

  it('maps issues found to warning styling', () => {
    const presentation = getPatrolRunStatusPresentation('issues_found');
    expect(presentation.badgeClass).toContain('amber-100');
    expect(presentation.label).toBe('issues found');
  });

  it('maps healthy runs to success styling', () => {
    expect(getPatrolRunStatusPresentation('healthy').badgeClass).toContain('green-100');
  });

  it('downgrades legacy healthy runs without findings snapshots to a neutral completed state', () => {
    expect(getPatrolRunStatusPresentation('healthy', 0, false)).toEqual({
      badgeClass: 'bg-surface-alt text-base-content',
      label: 'completed',
    });
  });

  it('treats erroring runs as error presentation even when the raw status says healthy', () => {
    const presentation = getPatrolRunStatusPresentation('healthy', 2);
    expect(presentation.label).toBe('error');
    expect(presentation.badgeClass).toContain('red-100');
  });

  it('treats only healthy runs without errors as healthy', () => {
    expect(isPatrolRunHealthy('healthy', 0)).toBe(true);
    expect(isPatrolRunHealthy('critical', 0)).toBe(false);
    expect(isPatrolRunHealthy('issues_found', 0)).toBe(false);
    expect(isPatrolRunHealthy('healthy', 1)).toBe(false);
  });

  it('normalizes unknown status labels safely', () => {
    const presentation = getPatrolRunStatusPresentation(' Needs Review ');
    expect(presentation.label).toBe('needs review');
    expect(presentation.badgeClass).toContain('bg-surface-alt');
  });

  it('maps tool call success and failure to canonical colors', () => {
    expect(getToolCallResultBadgeClass(true)).toContain('green-100');
    expect(getToolCallResultBadgeClass(false)).toContain('red-100');
    expect(getToolCallResultTextClass(true)).toContain('text-emerald-600');
    expect(getToolCallResultTextClass(false)).toContain('text-red-600');
  });

  it('returns canonical patrol run kind labels', () => {
    expect(getPatrolRunKindLabel('scoped')).toBe('Scoped run');
    expect(getPatrolRunKindLabel('patrol')).toBe('Full patrol');
    expect(getPatrolRunKindLabel('')).toBe('Full patrol');
  });

  it('expresses scoped run coverage against the effective scope', () => {
    expect(getPatrolRunCoverageSummary(scopedCoverageRun)).toBe(
      'Checked 1 of 2 scoped resources',
    );
    expect(getPatrolRunResourcesHeading(scopedCoverageRun)).toBe(
      'Resources checked (1 of 2 scoped)',
    );
  });

  it('uses checked-resource language for non-scoped coverage summaries', () => {
    expect(getPatrolRunCoverageSummary(fullCoverageRun)).toBe('Checked 58 resources');
    expect(getPatrolRunResourcesHeading(fullCoverageRun)).toBe('Resources checked (58)');
  });

  it('fails closed on zero-coverage scoped runs', () => {
    expect(
      getPatrolRunCoverageSummary({
        resources_checked: 0,
        scope_resource_ids: ['seed-resource'],
        effective_scope_resource_ids: ['expanded-a', 'expanded-b'],
      }),
    ).toBe('Checked 0 of 2 scoped resources');
  });

  it('returns canonical patrol run loading and unavailable copy', () => {
    expect(getRunHistoryLoadingState()).toBe('Loading run history…');
    expect(getToolCallsLoadingState()).toBe('Loading tool calls...');
    expect(getToolCallsUnavailableState()).toBe('Tool call details not available for this run.');
  });
});
