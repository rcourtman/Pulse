const RAW_TOOL_MARKERS = [
  '<｜DSML｜',
  '</｜DSML｜',
  '<｜｜DSML｜｜',
  '</｜｜DSML｜｜',
  '<｜/DSML｜',
  '<｜｜/DSML｜｜',
  '<|DSML|',
  '</|DSML|',
  '<||DSML||',
  '</||DSML||',
  '<|/DSML|',
  '<||/DSML||',
  '<tool_call',
  '</tool_call',
  '<tool_calls',
  '</tool_calls',
  '<function_call',
  '</function_call',
  '<function_calls',
  '</function_calls',
  '<|plugin|>',
  '<|interpreter|>',
  '<|tool_call|>',
];

const jsonToolCallLeakRe =
  /(?:^|\n)[ \t]*(?:```[ \t]*(?:json|JSON)?[ \t]*\n?[ \t]*)?\{[ \t\n]*"name"[ \t]*:[ \t]*"([a-zA-Z_][a-zA-Z0-9_]*)"/g;
const functionToolCallLeakRe = /(?:^|[^a-zA-Z0-9_])([a-zA-Z_][a-zA-Z0-9_]*)[ \t\r\n]*\(/g;
const minimaxToolCallLeakRe = /^minimax:tool_call\b/m;
const visibleInternalToolIdentifierRe =
  /`?\b((?:pulse|patrol)_[a-zA-Z0-9_]+|run_command)\b`?(?:[ \t]+(tool|command|query|call))?/g;
const internalReasoningHeadingRe =
  /^\s*(?:#{1,6}[ \t]*)?(thinking|thoughts?|reasoning|analysis)[ \t]*:?[ \t]*(?:\r?\n|$)/i;
const internalReasoningCueRe =
  /\b(?:we need to|i need to|let me|let's|need to|the user (?:asks|asked|is asking)|user question|before answering|before responding|tool call|call the|use the|inspect the prompt)\b/i;

const VISIBLE_INTERNAL_TOOL_IDENTIFIER_LABELS: Record<string, string> = {
  patrol_collect: 'collection',
  patrol_remediate: 'remediation',
  pulse_exec: 'command',
  pulse_query: 'inventory lookup',
  pulse_read: 'read command',
  pulse_run_command: 'command',
  run_command: 'command',
};

export interface AssistantOutputArtifactStreamState {
  visibleText: string;
  rawVisibleText: string;
  pendingText: string;
}

export function createAssistantOutputArtifactStreamState(): AssistantOutputArtifactStreamState {
  return {
    visibleText: '',
    rawVisibleText: '',
    pendingText: '',
  };
}

export function stripAssistantOutputArtifacts(content: string): {
  text: string;
  stripped: boolean;
} {
  const idx = assistantOutputArtifactIndex(content);
  if (idx < 0) {
    return { text: normalizeAssistantVisibleInternalIdentifiers(content), stripped: false };
  }
  const visiblePrefix = content.slice(0, idx).trim();
  return {
    text: shouldSuppressBeforeAssistantOutputArtifact(visiblePrefix)
      ? ''
      : normalizeAssistantVisibleInternalIdentifiers(visiblePrefix),
    stripped: true,
  };
}

export function appendVisibleTextBeforeAssistantOutputArtifacts(
  state: AssistantOutputArtifactStreamState,
  content: string,
): {
  text: string;
  stripped: boolean;
  previousVisibleText?: string;
  replacementText?: string;
} {
  if (!content && !state.pendingText) {
    return { text: '', stripped: false };
  }

  const text = state.pendingText + content;
  state.pendingText = '';

  const existingVisibleText = state.visibleText;
  const existingRawText = state.rawVisibleText || existingVisibleText;
  const candidate = existingRawText + text;
  const idx = assistantOutputArtifactIndex(candidate);
  if (idx < 0) {
    if (!existingRawText && shouldHoldPotentialAssistantOutputPrelude(text)) {
      state.pendingText = text;
      return { text: '', stripped: false };
    }

    const { visible, held } = splitTrailingPotentialToolNamePrefix(text);
    const normalizedVisible = normalizeAssistantVisibleInternalIdentifiers(visible);
    state.rawVisibleText += visible;
    state.visibleText += normalizedVisible;
    state.pendingText = held;
    return { text: normalizedVisible, stripped: false };
  }

  const safeText = candidate.slice(0, idx).trimEnd();
  if (shouldSuppressBeforeAssistantOutputArtifact(safeText)) {
    const previousVisibleText = state.visibleText;
    state.visibleText = '';
    state.rawVisibleText = '';
    state.pendingText = '';
    if (!previousVisibleText) {
      return {
        text: '',
        stripped: true,
      };
    }
    return {
      text: '',
      stripped: true,
      previousVisibleText,
      replacementText: '',
    };
  }
  const normalizedSafeText = normalizeAssistantVisibleInternalIdentifiers(safeText);
  const visibleDelta =
    normalizedSafeText.length > existingVisibleText.length
      ? normalizedSafeText.slice(existingVisibleText.length)
      : '';
  state.visibleText = normalizedSafeText;
  state.rawVisibleText = safeText;
  state.pendingText = '';
  return { text: visibleDelta, stripped: true };
}

export function flushPendingAssistantOutputText(state: AssistantOutputArtifactStreamState): string {
  if (!state.pendingText) {
    return '';
  }
  const text = state.pendingText;
  state.pendingText = '';
  if (shouldSuppressBeforeAssistantOutputArtifact(text)) {
    return '';
  }
  const normalizedText = normalizeAssistantVisibleInternalIdentifiers(text);
  state.rawVisibleText += text;
  state.visibleText += normalizedText;
  return normalizedText;
}

function normalizeAssistantVisibleInternalIdentifiers(content: string): string {
  if (!content) return content;

  return content.replace(visibleInternalToolIdentifierRe, (_match, rawName: string) => {
    const name = rawName.toLowerCase();
    const label = VISIBLE_INTERNAL_TOOL_IDENTIFIER_LABELS[name] || formatInternalIdentifier(name);
    return label;
  });
}

function assistantOutputArtifactIndex(content: string): number {
  if (!content) return -1;

  let first = -1;
  const record = (idx: number) => {
    if (idx < 0) return;
    if (first < 0 || idx < first) {
      first = idx;
    }
  };

  for (const marker of RAW_TOOL_MARKERS) {
    record(content.indexOf(marker));
  }

  record(findJSONToolCallLeak(content));
  record(findFunctionToolCallLeak(content));

  const minimaxMatch = minimaxToolCallLeakRe.exec(content);
  if (minimaxMatch) {
    record(minimaxMatch.index);
  }

  return first;
}

function findJSONToolCallLeak(content: string): number {
  for (const match of content.matchAll(jsonToolCallLeakRe)) {
    const name = match[1] || '';
    if (isAssistantToolLikeName(name)) {
      return match.index ?? -1;
    }
  }
  return -1;
}

function findFunctionToolCallLeak(content: string): number {
  for (const match of content.matchAll(functionToolCallLeakRe)) {
    const name = match[1] || '';
    if (isAssistantToolLikeName(name)) {
      return (match.index ?? 0) + match[0].lastIndexOf(name);
    }
  }
  return -1;
}

function splitTrailingPotentialToolNamePrefix(content: string): {
  visible: string;
  held: string;
} {
  if (!content) {
    return { visible: '', held: '' };
  }

  let start = content.length;
  while (start > 0 && isToolNameCharacter(content[start - 1])) {
    start -= 1;
  }

  if (start === content.length) {
    return { visible: content, held: '' };
  }

  const token = content.slice(start);
  if (isKnownAssistantToolNamePrefix(token)) {
    const holdStart = start > 0 && content[start - 1] === '`' ? start - 1 : start;
    return { visible: content.slice(0, holdStart), held: content.slice(holdStart) };
  }
  return { visible: content, held: '' };
}

function formatInternalIdentifier(name: string): string {
  return name.replace(/^(?:pulse|patrol)_/, '').replace(/_/g, ' ');
}

function isToolNameCharacter(char: string): boolean {
  return /[a-zA-Z0-9_]/.test(char);
}

function isAssistantToolLikeName(name: string): boolean {
  return /^(?:pulse|patrol)_[a-zA-Z0-9_]+$/.test(name);
}

function isKnownAssistantToolNamePrefix(prefix: string): boolean {
  if (!prefix) return false;
  return (
    'pulse_'.startsWith(prefix) ||
    'patrol_'.startsWith(prefix) ||
    /^pulse_[a-zA-Z0-9_]*$/.test(prefix) ||
    /^patrol_[a-zA-Z0-9_]*$/.test(prefix)
  );
}

function isCompactedToolPrelude(content: string): boolean {
  const trimmed = content.trim();
  if (!trimmed) return false;

  let letters = 0;
  let whitespace = 0;
  for (const char of trimmed) {
    if (/\p{L}/u.test(char)) {
      letters += 1;
    }
    if (/\s/.test(char)) {
      whitespace += 1;
    }
  }

  if (letters < 16) return false;
  if (whitespace === 0) return true;
  return letters >= 48 && whitespace <= 1;
}

function shouldSuppressBeforeAssistantOutputArtifact(content: string): boolean {
  return isCompactedToolPrelude(content) || isContentChannelReasoningPrelude(content);
}

function shouldHoldPotentialAssistantOutputPrelude(content: string): boolean {
  return isCompactedToolPrelude(content) || isPotentialContentChannelReasoningPrelude(content);
}

function isContentChannelReasoningPrelude(content: string): boolean {
  const trimmed = content.trim();
  if (!trimmed) return false;

  const match = trimmed.match(internalReasoningHeadingRe);
  if (!match) return false;

  const body = trimmed.slice(match[0].length).trim();
  if (!body) return true;
  if (internalReasoningCueRe.test(body)) return true;

  const words = body.split(/\s+/).filter(Boolean);
  return words.length >= 8;
}

function isPotentialContentChannelReasoningPrelude(content: string): boolean {
  const trimmed = content.trimStart();
  if (!trimmed) return false;

  const lower = trimmed.toLowerCase();
  if (lower.length < 'thinking'.length && 'thinking'.startsWith(lower)) return true;

  return internalReasoningHeadingRe.test(trimmed);
}
