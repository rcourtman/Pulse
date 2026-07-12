import { describe, expect, it } from 'vitest';
import type { AgentWorkflowPrompt } from '@/api/agentCapabilities';
import {
  getAssistantWorkflowStarters,
  getAssistantWorkflowStarterResourceId,
  getAssistantWorkflowStarterFindingId,
} from '../workflowStarters';

// Branch-coverage companion to workflowStarters.test.ts.
// workflowStarterLabel, resolveWorkflowStarterArgument and
// buildWorkflowStarterArguments are module-private, so each of their branches is
// driven through the public entry points (getAssistantWorkflowStarters for the
// builders; the two exported resolvers directly) and asserted against concrete
// shapes/strings — never truthiness-only.

const one = (prompts: AgentWorkflowPrompt[], context: Record<string, unknown> = {}) =>
  getAssistantWorkflowStarters(prompts, context);

// ---------------------------------------------------------------------------
// workflowStarterLabel (private) — reached via getAssistantWorkflowStarters .label
// ---------------------------------------------------------------------------

describe('workflowStarterLabel branch coverage', () => {
  it('returns the trimmed label when a label is present (normalizeIdentifier trim + truthy arm)', () => {
    const [starter] = one([
      { name: 'pulse_a', label: '   Spaced Label   ', arguments: [] },
    ]);
    expect(starter.label).toBe('Spaced Label');
  });

  it('falls through a non-string label (typeof !== "string" arm) to the description', () => {
    const [starter] = one([
      {
        name: 'pulse_a',
        label: 12345 as unknown as string,
        description: '   Use this description   ',
        arguments: [],
      },
    ]);
    expect(starter.label).toBe('Use this description');
  });

  it('falls through an empty-string label to the description branch', () => {
    const [starter] = one([
      { name: 'pulse_a', label: '', description: 'Desc only', arguments: [] },
    ]);
    expect(starter.label).toBe('Desc only');
  });

  it('title-cases the name when neither label nor description is present (name-formatting arm)', () => {
    const [starter] = one([{ name: 'pulse_triage_fleet', arguments: [] }]);
    // pulse_ stripped, underscores -> spaces, first letters uppercased.
    expect(starter.label).toBe('Triage Fleet');
  });

  it('title-cases a name that has no pulse_ prefix', () => {
    const [starter] = one([{ name: 'investigate_resource', arguments: [] }]);
    expect(starter.label).toBe('Investigate Resource');
  });

  it('title-cases a single-word name with no prefix and no underscores', () => {
    const [starter] = one([{ name: 'triage', arguments: [] }]);
    expect(starter.label).toBe('Triage');
  });
});

// ---------------------------------------------------------------------------
// getAssistantWorkflowStarterResourceId (exported)
// ---------------------------------------------------------------------------

describe('getAssistantWorkflowStarterResourceId branch coverage', () => {
  it('returns the first non-empty resource id, skipping a blank earlier entry', () => {
    expect(
      getAssistantWorkflowStarterResourceId({
        handoffResources: [{ id: '   ' }, { id: 'vm:2' }],
      }),
    ).toBe('vm:2');
  });

  it('skips a non-string resource id (typeof !== "string") and continues the loop', () => {
    expect(
      getAssistantWorkflowStarterResourceId({
        handoffResources: [{ id: 99 as unknown as string }],
      }),
    ).toBe('');
  });

  it('falls back to handoffActions targetResourceId when no resource id resolves', () => {
    expect(
      getAssistantWorkflowStarterResourceId({
        handoffActions: [{ targetResourceId: '' }, { targetResourceId: 'act-res' }],
      }),
    ).toBe('act-res');
  });

  it('falls back to targetId for a non-patrol target type when nothing else resolves', () => {
    expect(getAssistantWorkflowStarterResourceId({ targetType: 'vm', targetId: 'vm-99' })).toBe(
      'vm-99',
    );
  });

  it('rejects every patrol target type so a run/assessment/configuration id is not used', () => {
    expect(getAssistantWorkflowStarterResourceId({ targetType: 'patrol-run', targetId: 'run-1' })).toBe('');
    expect(
      getAssistantWorkflowStarterResourceId({ targetType: 'patrol-assessment', targetId: 'a-1' }),
    ).toBe('');
    expect(
      getAssistantWorkflowStarterResourceId({ targetType: 'patrol-configuration', targetId: 'c-1' }),
    ).toBe('');
  });

  it('returns empty string when no targetId is set, even with a targetType', () => {
    expect(getAssistantWorkflowStarterResourceId({})).toBe('');
    expect(getAssistantWorkflowStarterResourceId({ targetType: 'vm' })).toBe('');
  });

  it('returns a targetId whose targetType is absent (empty type is not a patrol type)', () => {
    expect(getAssistantWorkflowStarterResourceId({ targetId: 'only-id' })).toBe('only-id');
  });

  it('prefers a resource id over an action id and a target id', () => {
    expect(
      getAssistantWorkflowStarterResourceId({
        handoffResources: [{ id: 'r-1' }],
        handoffActions: [{ targetResourceId: 'a-1' }],
        targetType: 'vm',
        targetId: 't-1',
      }),
    ).toBe('r-1');
  });

  it('prefers an action id over a target id when no resource resolves', () => {
    expect(
      getAssistantWorkflowStarterResourceId({
        handoffActions: [{ targetResourceId: 'a-1' }],
        targetType: 'vm',
        targetId: 't-1',
      }),
    ).toBe('a-1');
  });
});

// ---------------------------------------------------------------------------
// getAssistantWorkflowStarterFindingId (exported)
// ---------------------------------------------------------------------------

describe('getAssistantWorkflowStarterFindingId branch coverage', () => {
  it('returns the direct findingId first', () => {
    expect(getAssistantWorkflowStarterFindingId({ findingId: 'f-direct' })).toBe('f-direct');
  });

  it('skips a non-string direct findingId and falls through to actions', () => {
    expect(
      getAssistantWorkflowStarterFindingId({
        findingId: 7 as unknown as string,
        handoffActions: [{ findingId: 'f-act' }],
      }),
    ).toBe('f-act');
  });

  it('returns the first non-empty action findingId, skipping a blank earlier entry', () => {
    expect(
      getAssistantWorkflowStarterFindingId({
        handoffActions: [{ findingId: '   ' }, { findingId: 'f-2' }],
      }),
    ).toBe('f-2');
  });

  it('falls back to context.context.findingId via the optional chain', () => {
    expect(getAssistantWorkflowStarterFindingId({ context: { findingId: 'f-ctx' } })).toBe(
      'f-ctx',
    );
  });

  it('returns empty string when context.context exists but has no findingId', () => {
    expect(getAssistantWorkflowStarterFindingId({ context: { other: 'x' } })).toBe('');
  });

  it('returns empty string when no source resolves (optional chain short-circuits)', () => {
    expect(getAssistantWorkflowStarterFindingId({})).toBe('');
    expect(getAssistantWorkflowStarterFindingId({ context: undefined })).toBe('');
  });
});

// ---------------------------------------------------------------------------
// resolveWorkflowStarterArgument (private) — reached via buildWorkflowStarterArguments
// ---------------------------------------------------------------------------

describe('resolveWorkflowStarterArgument branch coverage', () => {
  it('routes "resourceId" through getAssistantWorkflowStarterResourceId', () => {
    const [starter] = one(
      [{ name: 'p', arguments: [{ name: 'resourceId', required: true }] }],
      { handoffResources: [{ id: 'vm:7' }] },
    );
    expect(starter.arguments).toStrictEqual({ resourceId: 'vm:7' });
  });

  it('routes "finding_id" through getAssistantWorkflowStarterFindingId', () => {
    const [starter] = one(
      [{ name: 'p', arguments: [{ name: 'finding_id', required: true }] }],
      { findingId: 'f-1' },
    );
    expect(starter.arguments).toStrictEqual({ finding_id: 'f-1' });
  });

  it('returns undefined for an unknown argument name (default arm), omitting an optional arg', () => {
    const [starter] = one([{ name: 'p', arguments: [{ name: 'mystery', required: false }] }]);
    expect(starter.arguments).toStrictEqual({});
  });

  it('returns undefined for an unknown argument name and drops the prompt when the arg is required', () => {
    const starters = one([
      { name: 'known', label: 'Keep', arguments: [] },
      { name: 'dropped', label: 'Lose', arguments: [{ name: 'mystery', required: true }] },
    ]);
    expect(starters.map((s) => s.name)).toStrictEqual(['known']);
  });
});

// ---------------------------------------------------------------------------
// buildWorkflowStarterArguments (private) — reached via getAssistantWorkflowStarters
// ---------------------------------------------------------------------------

describe('buildWorkflowStarterArguments branch coverage', () => {
  it('treats a missing arguments array as empty and returns {}', () => {
    // No `arguments` field at all -> prompt.arguments ?? [] -> loop skipped.
    const [starter] = one([{ name: 'p', label: 'L' }] as AgentWorkflowPrompt[]);
    expect(starter.arguments).toStrictEqual({});
  });

  it('skips an argument whose name trims to empty', () => {
    const [starter] = one([
      { name: 'p', arguments: [{ name: '   ' }, { name: 'resourceId', required: false }] },
    ]);
    // Blank name is continue'd; resourceId optional with no value is omitted too.
    expect(starter.arguments).toStrictEqual({});
  });

  it('keeps a resolved optional argument and includes the prompt', () => {
    const [starter] = one(
      [{ name: 'p', arguments: [{ name: 'resourceId', required: false }] }],
      { handoffResources: [{ id: 'vm:1' }] },
    );
    expect(starter.arguments).toStrictEqual({ resourceId: 'vm:1' });
  });

  it('omits an unresolved optional argument but still includes the prompt with {}', () => {
    const [starter] = one([
      { name: 'p', arguments: [{ name: 'resourceId', required: false }] },
    ]);
    expect(starter).toMatchObject({ name: 'p' });
    expect(starter.arguments).toStrictEqual({});
  });

  it('returns null (drops the prompt) when a required argument has no value', () => {
    const starters = one([
      { name: 'kept', label: 'K', arguments: [] },
      { name: 'gone', label: 'G', arguments: [{ name: 'finding_id', required: true }] },
    ]);
    expect(starters.map((s) => s.name)).toStrictEqual(['kept']);
  });

  it('returns null when one optional arg resolves but a later required arg does not', () => {
    const starters = one(
      [
        {
          name: 'mixed',
          arguments: [
            { name: 'resourceId', required: false },
            { name: 'finding_id', required: true },
          ],
        },
      ],
      { handoffResources: [{ id: 'vm:1' }] },
    );
    expect(starters).toStrictEqual([]);
  });

  it('keeps the prompt when every required argument resolves', () => {
    const [starter] = one(
      [
        {
          name: 'all',
          arguments: [
            { name: 'resourceId', required: true },
            { name: 'finding_id', required: true },
          ],
        },
      ],
      { handoffResources: [{ id: 'vm:1' }], findingId: 'f-1' },
    );
    expect(starter.arguments).toStrictEqual({ resourceId: 'vm:1', finding_id: 'f-1' });
  });
});

// ---------------------------------------------------------------------------
// getAssistantWorkflowStarters (exported)
// ---------------------------------------------------------------------------

describe('getAssistantWorkflowStarters branch coverage', () => {
  it('skips a prompt whose name trims to empty', () => {
    const starters = one([
      { name: '', label: 'Empty', arguments: [] },
      { name: '   ', label: 'Blank', arguments: [] },
      { name: 'kept', label: 'Kept', arguments: [] },
    ]);
    expect(starters.map((s) => s.name)).toStrictEqual(['kept']);
  });

  it('skips a duplicate name, keeping the first occurrence', () => {
    const starters = one([
      { name: 'dup', label: 'First', arguments: [] },
      { name: 'dup', label: 'Second', arguments: [] },
    ]);
    expect(starters).toHaveLength(1);
    expect(starters[0].label).toBe('First');
  });

  it('trims a padded name for both id and name', () => {
    const [starter] = one([{ name: '   padded   ', label: 'L', arguments: [] }]);
    expect(starter.id).toBe('padded');
    expect(starter.name).toBe('padded');
  });

  it('sets description to undefined when the description trims to empty', () => {
    const [starter] = one([{ name: 'p', label: 'L', description: '    ', arguments: [] }]);
    expect(starter.description).toBeUndefined();
  });

  it('sets description to undefined when description is absent', () => {
    const [starter] = one([{ name: 'p', label: 'L', arguments: [] }]);
    expect(starter.description).toBeUndefined();
  });

  it('keeps a concrete description string when present', () => {
    const [starter] = one([{ name: 'p', label: 'L', description: 'A real desc', arguments: [] }]);
    expect(starter.description).toBe('A real desc');
  });

  it('falls back to the workflow kind when presentationKind is absent (normalize non-string arm)', () => {
    const [starter] = one([{ name: 'p', label: 'L', arguments: [] }] as AgentWorkflowPrompt[]);
    expect(starter.kind).toBe('workflow');
  });

  it('falls back to the workflow kind for an undeclared presentationKind string', () => {
    const [starter] = one([{ name: 'p', label: 'L', presentationKind: 'banana', arguments: [] }]);
    expect(starter.kind).toBe('workflow');
  });

  it('produces a fully-shaped starter for a valid kind with no description', () => {
    const starters = one([{ name: 'pulse_x', presentationKind: 'fleet', arguments: [] }]);
    expect(starters).toStrictEqual([
      {
        id: 'pulse_x',
        name: 'pulse_x',
        label: 'X',
        description: undefined,
        kind: 'fleet',
        arguments: {},
      },
    ]);
  });
});
