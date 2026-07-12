// Long pastes into the Assistant composer collapse into removable chips so a
// pasted log or config does not flood the textarea. The full text is appended
// to the typed prompt at send time; the chip is presentation-only state.

export interface PastedTextBlock {
  id: string;
  text: string;
  lineCount: number;
  charCount: number;
}

// A paste collapses when it is clearly a payload rather than a sentence the
// user would keep editing inline: many lines (logs, YAML, stack traces) or a
// long single-line blob (JSON, a command with args).
export const PASTE_COLLAPSE_MIN_LINES = 8;
export const PASTE_COLLAPSE_MIN_CHARS = 800;

export function shouldCollapsePastedText(text: string): boolean {
  if (!text.trim()) return false;
  if (text.length >= PASTE_COLLAPSE_MIN_CHARS) return true;
  return countLines(text) >= PASTE_COLLAPSE_MIN_LINES;
}

let pastedBlockCounter = 0;

export function createPastedTextBlock(text: string): PastedTextBlock {
  pastedBlockCounter += 1;
  return {
    id: `pasted-block-${pastedBlockCounter}`,
    text,
    lineCount: countLines(text),
    charCount: text.length,
  };
}

export function pastedBlockLabel(block: PastedTextBlock): string {
  if (block.lineCount > 1) {
    return `Pasted text (${block.lineCount.toLocaleString()} lines)`;
  }
  return `Pasted text (${block.charCount.toLocaleString()} chars)`;
}

// The typed prompt leads; pasted blocks follow in paste order, separated by
// blank lines, so the model reads the question before the payload.
export function composePromptWithPastedBlocks(
  typedPrompt: string,
  blocks: readonly PastedTextBlock[],
): string {
  const parts = [typedPrompt.trim(), ...blocks.map((block) => block.text.trim())].filter(Boolean);
  return parts.join('\n\n');
}

function countLines(text: string): number {
  return text.split('\n').length;
}
