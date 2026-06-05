const RAW_TOOL_MARKERS = [
  '<｜DSML｜',
  '</｜DSML｜',
  '<｜｜DSML｜｜',
  '</｜｜DSML｜｜',
  '<|DSML|',
  '</|DSML|',
  '<||DSML||',
  '</||DSML||',
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
  /(?:^|\n)[ \t]*(?:```[ \t]*(?:json|JSON)?[ \t]*\n?[ \t]*)?\{[ \t\n]*"name"[ \t]*:[ \t]*"((?:pulse|patrol)_[a-zA-Z0-9_]*)"/;
const functionToolCallLeakRe = /(?:^|[^a-zA-Z0-9_])((?:pulse|patrol)_[a-zA-Z0-9_]*)[ \t\r\n]*\(/;
const minimaxToolCallLeakRe = /^minimax:tool_call\b/m;

export function stripAssistantOutputArtifacts(content: string): {
  text: string;
  stripped: boolean;
} {
  const idx = assistantOutputArtifactIndex(content);
  if (idx < 0) {
    return { text: content, stripped: false };
  }
  return { text: content.slice(0, idx).trimEnd(), stripped: true };
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

  const jsonMatch = jsonToolCallLeakRe.exec(content);
  if (jsonMatch) {
    record(jsonMatch.index);
  }

  const functionMatch = functionToolCallLeakRe.exec(content);
  if (functionMatch?.[1]) {
    record(functionMatch.index + functionMatch[0].lastIndexOf(functionMatch[1]));
  }

  const minimaxMatch = minimaxToolCallLeakRe.exec(content);
  if (minimaxMatch) {
    record(minimaxMatch.index);
  }

  return first;
}
