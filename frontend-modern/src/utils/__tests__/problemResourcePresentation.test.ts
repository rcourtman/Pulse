import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getProblemResourceIssueLabel,
  getProblemResourceMemberLabel,
  getProblemResourceStatusVariant,
  isGenericProblemResourceDisplayName,
} from '@/utils/problemResourcePresentation';

function resource(overrides: Partial<Resource>): Resource {
  return {
    id: 'resource-1',
    type: 'storage',
    name: 'storage',
    displayName: 'storage',
    status: 'offline',
    ...overrides,
  } as Resource;
}

describe('problemResourcePresentation', () => {
  it('treats degraded dashboard problem resources as warning', () => {
    expect(getProblemResourceStatusVariant(149)).toBe('warning');
  });

  it('treats offline or critical dashboard problem resources as danger', () => {
    expect(getProblemResourceStatusVariant(150)).toBe('danger');
    expect(getProblemResourceStatusVariant(200)).toBe('danger');
  });

  it('normalizes generic type-plus-status resource names to an issue label', () => {
    const problemResource = resource({
      id: 'storage-offline',
      name: 'storage (offline)',
      displayName: 'storage (offline)',
    });

    expect(isGenericProblemResourceDisplayName(problemResource, ['Offline'])).toBe(true);
    expect(getProblemResourceIssueLabel(problemResource, ['Offline'])).toBe('the storage issue');
    expect(getProblemResourceMemberLabel(problemResource, ['Offline'])).toBeNull();
  });

  it('preserves meaningful resource names for problem rows', () => {
    const problemResource = resource({
      id: 'vm-101',
      type: 'vm',
      name: 'database-vm',
      displayName: 'database-vm',
    });

    expect(isGenericProblemResourceDisplayName(problemResource, ['Offline'])).toBe(false);
    expect(getProblemResourceIssueLabel(problemResource, ['Offline'])).toBe('database-vm');
    expect(getProblemResourceMemberLabel(problemResource, ['Offline'])).toBe('database-vm');
  });
});
