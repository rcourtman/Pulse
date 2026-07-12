import { describe, expect, it } from 'vitest';
import {
  appendVisibleTextBeforeAssistantOutputArtifacts,
  createAssistantOutputArtifactStreamState,
  flushPendingAssistantOutputText,
  stripAssistantOutputArtifacts,
} from '../assistantOutputHygiene';

// Branch-coverage companion to assistantOutputHygiene.test.ts. Every target
// function below is module-private, so each branch is driven through the public
// entry points (stripAssistantOutputArtifacts / appendVisibleTextBefore… /
// flushPendingAssistantOutputText) and asserted against concrete shapes.

const freshState = () => createAssistantOutputArtifactStreamState();

// ---------------------------------------------------------------------------
// formatInternalIdentifier — reached via normalizeAssistantVisibleInternalIdentifiers
// when an identifier is NOT in VISIBLE_INTERNAL_TOOL_IDENTIFIER_LABELS.
// ---------------------------------------------------------------------------

describe('formatInternalIdentifier branch coverage', () => {
  it('strips a pulse_ prefix and turns underscores into spaces', () => {
    // pulse_get_nodes is not in the labels map -> formatInternalIdentifier.
    expect(stripAssistantOutputArtifacts('Use pulse_get_nodes now.')).toEqual({
      text: 'Use get nodes now.',
      stripped: false,
    });
  });

  it('strips a patrol_ prefix leaving a single word with no underscores', () => {
    expect(stripAssistantOutputArtifacts('Run patrol_scan.')).toEqual({
      text: 'Run scan.',
      stripped: false,
    });
  });

  it('strips a patrol_ prefix and keeps internal underscores as spaces', () => {
    expect(stripAssistantOutputArtifacts('Start patrol_network_scan.')).toEqual({
      text: 'Start network scan.',
      stripped: false,
    });
  });
});

// ---------------------------------------------------------------------------
// normalizeAssistantVisibleInternalIdentifiers
// ---------------------------------------------------------------------------

describe('normalizeAssistantVisibleInternalIdentifiers branch coverage', () => {
  it('returns empty input untouched via the early guard', () => {
    // stripAssistantOutputArtifacts('') -> idx<0 -> normalize('') -> ''.
    expect(stripAssistantOutputArtifacts('')).toEqual({ text: '', stripped: false });
  });

  it('leaves prose without any internal identifier untouched', () => {
    expect(stripAssistantOutputArtifacts('Just regular prose.')).toEqual({
      text: 'Just regular prose.',
      stripped: false,
    });
  });

  it('substitutes a mapped label (patrol_collect -> collection)', () => {
    expect(stripAssistantOutputArtifacts('The patrol_collect ran.')).toEqual({
      text: 'The collection ran.',
      stripped: false,
    });
  });

  it('consumes surrounding backticks and a trailing label word, handling multiple matches', () => {
    // The regex swallows the optional backticks and the trailing
    // (tool|command|query|call) word, replacing the whole match with the label.
    expect(
      stripAssistantOutputArtifacts('`pulse_read` tool and run_command query'),
    ).toEqual({
      text: 'read command and command',
      stripped: false,
    });
  });
});

// ---------------------------------------------------------------------------
// appendVisibleTextBeforeAssistantOutputArtifacts
// ---------------------------------------------------------------------------

describe('appendVisibleTextBeforeAssistantOutputArtifacts branch coverage', () => {
  it('short-circuits when both content and pendingText are empty', () => {
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(freshState(), '')).toEqual({
      text: '',
      stripped: false,
    });
  });

  it('emits previousVisibleText + replacementText when prior text is suppressed at an artifact', () => {
    const state = freshState();
    // First delta establishes non-empty visibleText.
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Hello.')).toEqual({
      text: 'Hello.',
      stripped: false,
    });
    // Second delta is a compacted prelude (0 whitespace, >=16 letters) glued to
    // a pulse_read leak. shouldSuppress is true AND previousVisibleText is set.
    // The '\n' separator before pulse_read is required: findFunctionToolCallLeak
    // only matches a tool name preceded by a non-word character.
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        'Illcheckthedevicenodesinsidethecontainertoanswerthat\npulse_read(target_host="x")',
      ),
    ).toEqual({
      text: '',
      stripped: true,
      previousVisibleText: 'Hello.',
      replacementText: '',
    });
    expect(state.visibleText).toBe('');
    expect(state.rawVisibleText).toBe('');
    expect(state.pendingText).toBe('');
  });

  it('returns a non-empty visibleDelta when new safe text grows beyond what was shown', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Hello.')).toEqual({
      text: 'Hello.',
      stripped: false,
    });
    // The artifact appears after additional visible prose, so normalizedSafeText
    // is longer than existingVisibleText -> visibleDelta is the new slice.
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        ' World.\npulse_read(target_host="x")',
      ),
    ).toEqual({ text: ' World.', stripped: true });
  });

  it('returns an empty visibleDelta when the re-sliced safe text does not grow', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Done.')).toEqual({
      text: 'Done.',
      stripped: false,
    });
    // safeText re-slices back to 'Done.' which is no longer than what was shown.
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        '\npulse_read(target_host="x")',
      ),
    ).toEqual({ text: '', stripped: true });
  });
});

// ---------------------------------------------------------------------------
// flushPendingAssistantOutputText
// ---------------------------------------------------------------------------

describe('flushPendingAssistantOutputText branch coverage', () => {
  it('returns an empty string when there is no pending text', () => {
    expect(flushPendingAssistantOutputText(freshState())).toBe('');
  });

  it('releases a held partial-reasoning token (not suppressible) and updates state', () => {
    const state = freshState();
    // 'Think' is held as a potential reasoning prelude (prefix of "thinking").
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Think')).toEqual({
      text: '',
      stripped: false,
    });
    // On flush it is NOT a complete reasoning heading -> not suppressed.
    expect(flushPendingAssistantOutputText(state)).toBe('Think');
    expect(state.visibleText).toBe('Think');
    expect(state.rawVisibleText).toBe('Think');
    expect(state.pendingText).toBe('');
  });

  it('suppresses a held compacted prelude on flush', () => {
    const state = freshState();
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(
        state,
        'Thisisbadmodelspacingbutitistheactualanswerbecauseitneverturnsintoatoolcall.',
      ),
    ).toEqual({ text: '', stripped: false });
    expect(flushPendingAssistantOutputText(state)).toBe('');
    expect(state.visibleText).toBe('');
  });

  it('suppresses a held complete reasoning heading on flush', () => {
    const state = freshState();
    // 'Thinking' alone is a complete heading -> suppressible on flush.
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Thinking')).toEqual({
      text: '',
      stripped: false,
    });
    expect(flushPendingAssistantOutputText(state)).toBe('');
  });
});

// ---------------------------------------------------------------------------
// isContentChannelReasoningPrelude
// ---------------------------------------------------------------------------

describe('isContentChannelReasoningPrelude branch coverage', () => {
  it('returns false for empty content (reached via an empty visible prefix)', () => {
    // visiblePrefix trims to '' -> shouldSuppress false -> normalize('') shown.
    expect(stripAssistantOutputArtifacts('pulse_read(target_host="x")')).toEqual({
      text: '',
      stripped: true,
    });
  });

  it('returns false when no reasoning heading is present', () => {
    expect(stripAssistantOutputArtifacts('Just prose here\npulse_read()')).toEqual({
      text: 'Just prose here',
      stripped: true,
    });
  });

  it('returns true for a bare heading with no body', () => {
    expect(stripAssistantOutputArtifacts('Thinking\npulse_read()')).toEqual({
      text: '',
      stripped: true,
    });
  });

  it('returns true when the body contains a reasoning cue', () => {
    // cue "let me" -> internalReasoningCueRe matches -> suppress.
    expect(
      stripAssistantOutputArtifacts('Thoughts\nlet me count the entries first\npulse_read()'),
    ).toEqual({ text: '', stripped: true });
  });

  it('returns true for a cue-less body with >= 8 words', () => {
    expect(
      stripAssistantOutputArtifacts(
        'Thinking\nthe quick brown fox jumps over the lazy dog now\npulse_read()',
      ),
    ).toEqual({ text: '', stripped: true });
  });

  it('returns false for a cue-less body with < 8 words', () => {
    expect(stripAssistantOutputArtifacts('Thinking\nshort body here\npulse_read()')).toEqual({
      text: 'Thinking\nshort body here',
      stripped: true,
    });
  });
});

// ---------------------------------------------------------------------------
// isPotentialContentChannelReasoningPrelude — reached via the append hold path.
// ---------------------------------------------------------------------------

describe('isPotentialContentChannelReasoningPrelude branch coverage', () => {
  it('holds a short prefix of "thinking" (partial-prefix true arm)', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'Think')).toEqual({
      text: '',
      stripped: false,
    });
    expect(state.pendingText).toBe('Think');
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

  it('does not hold a short non-prefix token (partial-prefix false + heading false)', () => {
    const state = freshState();
    // 'xyz' is < 8 chars but not a prefix of "thinking" and not a heading.
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'xyz')).toEqual({
      text: 'xyz',
      stripped: false,
    });
    expect(state.pendingText).toBe('');
  });
});

// ---------------------------------------------------------------------------
// isCompactedToolPrelude
// ---------------------------------------------------------------------------

describe('isCompactedToolPrelude branch coverage', () => {
  it('returns false for letters < 16 (short prose before a leak)', () => {
    expect(stripAssistantOutputArtifacts('Hi.pulse_read()')).toEqual({
      text: 'Hi.',
      stripped: true,
    });
  });

  it('returns true for >= 16 letters with zero whitespace', () => {
    expect(
      stripAssistantOutputArtifacts(
        'Illcheckthedevicenodesinsidethecontainertoanswerthat\npulse_read()',
      ),
    ).toEqual({ text: '', stripped: true });
  });

  it('returns true for >= 48 letters with exactly one whitespace', () => {
    // 43 letters + 1 space + 5 letters = 48 letters, 1 whitespace.
    expect(
      stripAssistantOutputArtifacts(
        'thequickbrownfoxjumpsoverlazydogandrunsfast abcde\npulse_read()',
      ),
    ).toEqual({ text: '', stripped: true });
  });

  it('returns false for 16-47 letters with exactly one whitespace', () => {
    // 20 letters + 1 space + 1 letter = 21 letters, 1 whitespace.
    expect(
      stripAssistantOutputArtifacts('aaaaaaaaaaaaaaaaaaaa b\npulse_read()'),
    ).toEqual({ text: 'aaaaaaaaaaaaaaaaaaaa b', stripped: true });
  });

  it('returns false when whitespace >= 2 even with many letters', () => {
    expect(
      stripAssistantOutputArtifacts('alpha beta gamma delta epsilon zeta\npulse_read()'),
    ).toEqual({ text: 'alpha beta gamma delta epsilon zeta', stripped: true });
  });
});

// ---------------------------------------------------------------------------
// splitTrailingPotentialToolNamePrefix
// ---------------------------------------------------------------------------

describe('splitTrailingPotentialToolNamePrefix branch coverage', () => {
  it('holds nothing when the trailing run is not a known tool prefix', () => {
    // 'world123' is tool-name characters but not a pulse/patrol prefix.
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(freshState(), 'hello world123')).toEqual({
      text: 'hello world123',
      stripped: false,
    });
  });

  it('keeps a leading backtick attached to a held pulse_ prefix', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'see `pulse_')).toEqual({
      text: 'see ',
      stripped: false,
    });
    expect(state.pendingText).toBe('`pulse_');
  });

  it('holds a known patrol_ partial prefix without a backtick', () => {
    const state = freshState();
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'go pat')).toEqual({
      text: 'go ',
      stripped: false,
    });
    expect(state.pendingText).toBe('pat');
  });
});

// ---------------------------------------------------------------------------
// isKnownAssistantToolNamePrefix
// ---------------------------------------------------------------------------

describe('isKnownAssistantToolNamePrefix branch coverage', () => {
  it('matches a complete pulse_ tool name via the pulse_ regex arm', () => {
    const state = freshState();
    // 'pulse_read' (no parens) is a known prefix -> held, then normalized on flush.
    expect(appendVisibleTextBeforeAssistantOutputArtifacts(state, 'use pulse_read')).toEqual({
      text: 'use ',
      stripped: false,
    });
    expect(state.pendingText).toBe('pulse_read');
    expect(flushPendingAssistantOutputText(state)).toBe('read command');
  });

  it('matches a complete patrol_ tool name via the patrol_ regex arm', () => {
    const state = freshState();
    expect(
      appendVisibleTextBeforeAssistantOutputArtifacts(state, 'try patrol_remediate'),
    ).toEqual({
      text: 'try ',
      stripped: false,
    });
    expect(state.pendingText).toBe('patrol_remediate');
    // patrol_remediate IS in the labels map -> "remediation" on flush.
    expect(flushPendingAssistantOutputText(state)).toBe('remediation');
  });
});

// ---------------------------------------------------------------------------
// assistantOutputArtifactIndex
// ---------------------------------------------------------------------------

describe('assistantOutputArtifactIndex branch coverage', () => {
  it('returns -1 for plain text with no artifacts', () => {
    expect(stripAssistantOutputArtifacts('plain text only')).toEqual({
      text: 'plain text only',
      stripped: false,
    });
  });

  it('detects a minimax:tool_call leak', () => {
    // minimaxToolCallLeakRe matches at start of the second line.
    expect(stripAssistantOutputArtifacts('Hello\nminimax:tool_call foo')).toEqual({
      text: 'Hello',
      stripped: true,
    });
  });

  it('keeps the earliest marker when a later-scanned raw marker appears first', () => {
    // '<｜DSML｜' is scanned before '<tool_call' but appears later in the string.
    // record() must lower `first` from the <｜DSML｜ index down to <tool_call's 2.
    expect(stripAssistantOutputArtifacts('Hi<tool_call></tool_call><｜DSML｜')).toEqual({
      text: 'Hi',
      stripped: true,
    });
  });

  it('detects a fenced ```json tool-call leak', () => {
    expect(stripAssistantOutputArtifacts('```json\n{"name":"pulse_exec","a":1}\n```')).toEqual({
      text: '',
      stripped: true,
    });
  });
});

// ---------------------------------------------------------------------------
// findJSONToolCallLeak
// ---------------------------------------------------------------------------

describe('findJSONToolCallLeak branch coverage', () => {
  it('skips a JSON object whose name is not tool-like and finds no other artifact', () => {
    // 'helper' fails isAssistantToolLikeName -> loop continues -> returns -1.
    expect(stripAssistantOutputArtifacts('{"name":"helper","x":1}')).toEqual({
      text: '{"name":"helper","x":1}',
      stripped: false,
    });
  });

  it('returns the index of a tool-like JSON leak at the start of the content', () => {
    expect(
      stripAssistantOutputArtifacts('{"name":"pulse_query","parameters":{"action":"list"}}'),
    ).toEqual({ text: '', stripped: true });
  });
});

// ---------------------------------------------------------------------------
// findFunctionToolCallLeak
// ---------------------------------------------------------------------------

describe('findFunctionToolCallLeak branch coverage', () => {
  it('reports the tool name position, not the preceding non-word character', () => {
    // 'pulse_read' is preceded by a space; the leak index must point at 'p'.
    expect(stripAssistantOutputArtifacts('Run pulse_read(target_host="x") now')).toEqual({
      text: 'Run',
      stripped: true,
    });
  });

  it('skips a non-tool-like function call so no artifact is reported', () => {
    expect(stripAssistantOutputArtifacts('Call helper(target="x") in the example.')).toEqual({
      text: 'Call helper(target="x") in the example.',
      stripped: false,
    });
  });
});
