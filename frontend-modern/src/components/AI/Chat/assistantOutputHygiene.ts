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

export interface AssistantOutputArtifactStreamState {
  visibleText: string;
  pendingText: string;
}

export function createAssistantOutputArtifactStreamState(): AssistantOutputArtifactStreamState {
  return {
    visibleText: '',
    pendingText: '',
  };
}

export function stripAssistantOutputArtifacts(content: string): {
  text: string;
  stripped: boolean;
} {
  const idx = assistantOutputArtifactIndex(content);
  if (idx < 0) {
    return { text: content, stripped: false };
  }
  const visiblePrefix = content.slice(0, idx).trim();
  return {
    text: isCompactedToolPrelude(visiblePrefix) ? '' : visiblePrefix,
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

  const existing = state.visibleText;
  const candidate = existing + text;
  const idx = assistantOutputArtifactIndex(candidate);
  if (idx < 0) {
    const { visible, held } = splitTrailingPotentialToolNamePrefix(text);
    state.visibleText += visible;
    state.pendingText = held;
    return { text: visible, stripped: false };
  }

  const safeText = candidate.slice(0, idx).trimEnd();
  if (isCompactedToolPrelude(safeText)) {
    const previousVisibleText = state.visibleText;
    state.visibleText = '';
    state.pendingText = '';
    return {
      text: '',
      stripped: true,
      previousVisibleText,
      replacementText: '',
    };
  }
  const visibleDelta = safeText.length > existing.length ? safeText.slice(existing.length) : '';
  state.visibleText = safeText;
  state.pendingText = '';
  return { text: visibleDelta, stripped: true };
}

export function flushPendingAssistantOutputText(state: AssistantOutputArtifactStreamState): string {
  if (!state.pendingText) {
    return '';
  }
  const text = state.pendingText;
  state.pendingText = '';
  state.visibleText += text;
  return text;
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
    return { visible: content.slice(0, start), held: token };
  }
  return { visible: content, held: '' };
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
