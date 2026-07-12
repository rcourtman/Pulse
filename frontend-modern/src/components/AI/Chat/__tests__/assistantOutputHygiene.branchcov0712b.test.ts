import { describe, expect, it } from 'vitest';
import {
  appendVisibleTextBeforeAssistantOutputArtifacts,
  createAssistantOutputArtifactStreamState,
  flushPendingAssistantOutputText,
  stripAssistantOutputArtifacts,
} from '../assistantOutputHygiene';

// Branch-coverage companion to assistantOutputHygiene.test.ts. The five target
// functions (findJSONToolCallLeak, findFunctionToolCallLeak,
// splitTrailingPotentialToolNamePrefix, isKnownAssistantToolNamePrefix,
// isPotentialContentChannelReasoningPrelude) are all module-private, so every
// branch is driven through the three public entry points and asserted against
// concrete shapes/strings.

const freshState = () => createAssistantOutputArtifactStreamState();

// ---------------------------------------------------------------------------
// findJSONToolCallLeak — reached via assistantOutputArtifactIndex, surfaced
// through stripAssistantOutputArtifacts.
// ---------------------------------------------------------------------------

describe('findJSONToolCallLeak branch coverage', () => {
  it('returns -1 (no artifact) when the JSON object name is not tool-like', () => {
    // name 'helper' fails isAssistantToolLikeName -> loop yields no leak, and no
    // other artifact marker is present, so the content is preserved verbatim.
    expect(stripAssistantOutputArtifacts('{"name":"helper","x":1}')).toEqual({
      text: '{"name":"helper","x":1}',
      stripped: false,
    });
  });

  it('returns -1 when no JSON tool-call object appears at all', () => {
    expect(stripAssistantOutputArtifacts('just plain prose here')).toEqual({
      text: 'just plain prose here',
      stripped: false,
    });
  });

  it('reports the artifact when the JSON name is tool-like via the fenced ```json arm', () => {
    expect(stripAssistantOutputArtifacts('```json\n{"name":"pulse_exec","a":1}\n```')).toEqual({
      text: '',
      stripped: true,
    });
  });

  it('continues past a non-tool JSON match and reports a later tool-like match', () => {
    // First object 'helper' is skipped (isAssistantToolLikeName false -> loop
    // continues); the second object 'pulse_query' matches -> leak index returned.
    expect(
      stripAssistantOutputArtifacts('{"name":"helper","x":1}\n{"name":"pulse_query"}'),
    ).toEqual({
      text: '{"name":"helper","x":1}',
      stripped: true,
    });
  });
});

// ---------------------------------------------------------------------------
// findFunctionToolCallLeak — reached via assistantOutputArtifactIndex.
// ---------------------------------------------------------------------------

describe('findFunctionToolCallLeak branch coverage', () => {
  it('returns -1 (no artifact) when a function-call name is not tool-like', () => {
    expect(stripAssistantOutputArtifacts('Call helper(target="x") in the demo.')).toEqual({
      text: 'Call helper(target="x") in the demo.',
      stripped: false,
    });
  });

  it('reports the tool NAME position rather than the preceding non-word character', () => {
    // 'pulse_read' is preceded by a space; the leak index must resolve to the
    // 'p', so the visible prefix slices exactly to 'Run'.
    expect(stripAssistantOutputArtifacts('Run pulse_read(target_host="x") now')).toEqual({
      text: 'Run',
      stripped: true,
    });
  });

  it('continues past a non-tool function call and reports a later tool-like call', () => {
    // 'helper(' is skipped; 'pulse_read(' matches -> the visible prefix is the
    // prose before the tool name.
    expect(stripAssistantOutputArtifacts('helper(x) pulse_read(y)')).toEqual({
      text: 'helper(x)',
      stripped: true,
    });
  });
});

// ---------------------------------------------------------------------------
// splitTrailingPotentialToolNamePrefix — reached via the append() idx<0 path.
// ---------------------------------------------------------------------------

describe('splitTrailingPotentialToolNamePrefix branch coverage', () => {
  it('emits all text with nothing held when there is no trailing tool-name run', () => {
    // 'Hello.' ends in '.' (not a tool-name char) -> start === length arm.
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Hello.')).toEqual({
      text: 'Hello.',
      stripped: false,
    });
    expect(state.pendingText).toBe('');
  });

  it('holds a trailing pulse_ prefix together with its preceding backtick', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'see `pulse_')).toEqual({
      text: 'see ',
      stripped: false,
    });
    expect(state.pendingText).toBe('`pulse_');
  });

  it('holds a trailing patrol partial prefix with no backtick', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'go pat')).toEqual({
      text: 'go ',
      stripped: false,
    });
    expect(state.pendingText).toBe('pat');
  });

  it('holds nothing when the trailing run is not a known tool prefix', () => {
    // 'pulx' is a tool-name run but not a prefix of pulse_/patrol_ and matches
    // neither regex -> token-not-known arm.
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'go pulx')).toEqual({
      text: 'go pulx',
      stripped: false,
    });
    expect(state.pendingText).toBe('');
  });
});

// ---------------------------------------------------------------------------
// isKnownAssistantToolNamePrefix — reached via splitTrailingPotentialToolNamePrefix.
// ---------------------------------------------------------------------------

describe('isKnownAssistantToolNamePrefix branch coverage', () => {
  it('matches a bare pulse partial prefix via the "pulse_".startsWith arm', () => {
    // 'pulse' (no underscore) is a prefix of 'pulse_' -> startsWith arm true.
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'use pulse')).toEqual({
      text: 'use ',
      stripped: false,
    });
    expect(state.pendingText).toBe('pulse');
  });

  it('matches a full pulse_ tool name via the /^pulse_/ regex arm', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'use pulse_read')).toEqual({
      text: 'use ',
      stripped: false,
    });
    expect(state.pendingText).toBe('pulse_read');
    // On flush the held token is normalized through the labels map.
    expect(flushPendingAssistantOutputText(state)).toBe('read command');
  });

  it('matches a full patrol_ tool name via the /^patrol_/ regex arm', () => {
    const state = freshState();
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(state, 'try patrol_remediate'),
    ).toEqual({
      text: 'try ',
      stripped: false,
    });
    expect(state.pendingText).toBe('patrol_remediate');
    expect(flushPendingAssistantOutputText(state)).toBe('remediation');
  });
});

// ---------------------------------------------------------------------------
// isPotentialContentChannelReasoningPrelude — reached via the append() hold path
// (shouldHoldPotentialAssistantOutputPrelude) when rawVisibleText is empty.
// ---------------------------------------------------------------------------

describe('isPotentialContentChannelReasoningPrelude branch coverage', () => {
  it('returns false for whitespace-only content (empty-trimmed early guard)', () => {
    // '   ' trimStarts to '' -> !trimmed -> false, so nothing is held and the
    // spaces flow through unchanged.
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, '   ')).toEqual({
      text: '   ',
      stripped: false,
    });
    expect(state.pendingText).toBe('');
  });

  it('holds a short lowercase prefix of "thinking" (partial-prefix true arm)', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Think')).toEqual({
      text: '',
      stripped: false,
    });
    expect(state.pendingText).toBe('Think');
  });

  it('holds a mixed-case prefix via the toLowerCase + partial-prefix arm', () => {
    // 'THINK'.toLowerCase() === 'think' is still a prefix of 'thinking'.
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'THINK')).toEqual({
      text: '',
      stripped: false,
    });
    expect(state.pendingText).toBe('THINK');
  });

  it('holds the full "thinking" token via the heading arm (length is not < 8)', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Thinking')).toEqual({
      text: '',
      stripped: false,
    });
    expect(state.pendingText).toBe('Thinking');
  });

  it('holds a "reasoning" heading via the heading arm', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Reasoning')).toEqual({
      text: '',
      stripped: false,
    });
    expect(state.pendingText).toBe('Reasoning');
  });

  it('holds a markdown-prefixed reasoning heading (the optional #{1,6} arm)', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, '## Thinking')).toEqual({
      text: '',
      stripped: false,
    });
    expect(state.pendingText).toBe('## Thinking');
  });

  it('does not hold a short token that is neither a thinking-prefix nor a heading', () => {
    // 'xyz' length < 8 but not a prefix of 'thinking' (partial arm false) and not
    // a heading (heading arm false) -> not held.
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'xyz')).toEqual({
      text: 'xyz',
      stripped: false,
    });
    expect(state.pendingText).toBe('');
  });

  it('does not hold a longer token whose length skips the partial arm but is no heading', () => {
    // 'hello world' length >= 8 so the partial-prefix arm is skipped, and it is
    // not a reasoning heading -> not held.
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'hello world')).toEqual({
      text: 'hello world',
      stripped: false,
    });
    expect(state.pendingText).toBe('');
  });
});
