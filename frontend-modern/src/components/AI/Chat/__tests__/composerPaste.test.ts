import { describe, expect, it } from 'vitest';
import {
  PASTE_COLLAPSE_MIN_CHARS,
  PASTE_COLLAPSE_MIN_LINES,
  composePromptWithPastedBlocks,
  createPastedTextBlock,
  pastedBlockLabel,
  shouldCollapsePastedText,
} from '../composerPaste';

describe('composerPaste', () => {
  describe('shouldCollapsePastedText', () => {
    it('ignores empty and whitespace-only pastes', () => {
      expect(shouldCollapsePastedText('')).toBe(false);
      expect(shouldCollapsePastedText('   \n\n  ')).toBe(false);
    });

    it('keeps short pastes inline', () => {
      expect(shouldCollapsePastedText('restart the vm please')).toBe(false);
      expect(shouldCollapsePastedText('line one\nline two\nline three')).toBe(false);
    });

    it('collapses many-line pastes', () => {
      const lines = Array.from({ length: PASTE_COLLAPSE_MIN_LINES }, (_, i) => `log line ${i}`);
      expect(shouldCollapsePastedText(lines.join('\n'))).toBe(true);
      expect(shouldCollapsePastedText(lines.slice(0, -1).join('\n'))).toBe(false);
    });

    it('collapses long single-line pastes', () => {
      expect(shouldCollapsePastedText('x'.repeat(PASTE_COLLAPSE_MIN_CHARS))).toBe(true);
      expect(shouldCollapsePastedText('x'.repeat(PASTE_COLLAPSE_MIN_CHARS - 1))).toBe(false);
    });
  });

  describe('createPastedTextBlock and pastedBlockLabel', () => {
    it('counts lines and chars and labels multi-line blocks by lines', () => {
      const block = createPastedTextBlock('a\nb\nc');
      expect(block.lineCount).toBe(3);
      expect(block.charCount).toBe(5);
      expect(pastedBlockLabel(block)).toBe('Pasted text (3 lines)');
    });

    it('labels single-line blocks by chars', () => {
      const block = createPastedTextBlock('y'.repeat(1200));
      expect(pastedBlockLabel(block)).toBe(`Pasted text (${(1200).toLocaleString()} chars)`);
    });

    it('assigns unique ids', () => {
      expect(createPastedTextBlock('a').id).not.toBe(createPastedTextBlock('a').id);
    });
  });

  describe('composePromptWithPastedBlocks', () => {
    it('returns the typed prompt untouched without blocks', () => {
      expect(composePromptWithPastedBlocks('hello', [])).toBe('hello');
    });

    it('appends blocks after the typed prompt separated by blank lines', () => {
      const blocks = [createPastedTextBlock('log a\nlog b'), createPastedTextBlock('log c')];
      expect(composePromptWithPastedBlocks('what broke?', blocks)).toBe(
        'what broke?\n\nlog a\nlog b\n\nlog c',
      );
    });

    it('sends blocks alone when nothing was typed', () => {
      const blocks = [createPastedTextBlock('  raw payload  ')];
      expect(composePromptWithPastedBlocks('', blocks)).toBe('raw payload');
    });
  });
});
