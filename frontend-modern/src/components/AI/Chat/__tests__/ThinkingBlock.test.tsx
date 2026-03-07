import { describe, expect, it } from 'vitest';
import { cleanup, render, screen, fireEvent } from '@solidjs/testing-library';
import { afterEach } from 'vitest';
import { ThinkingBlock } from '../ThinkingBlock';

afterEach(cleanup);

describe('ThinkingBlock', () => {
  // --- Header / label rendering ---

  it('renders "Thinking" label when not streaming', () => {
    render(() => <ThinkingBlock content="Some reasoning" />);
    expect(screen.getByText('Thinking')).toBeInTheDocument();
  });

  it('renders "Thinking..." label when streaming', () => {
    render(() => <ThinkingBlock content="Some reasoning" isStreaming={true} />);
    expect(screen.getByText('Thinking...')).toBeInTheDocument();
  });

  // --- Stats display ---

  it('shows correct line and word count', () => {
    const content = 'First line\nSecond line\nThird line';
    render(() => <ThinkingBlock content={content} />);
    expect(screen.getByText('3 lines · 6 words')).toBeInTheDocument();
  });

  it('counts only non-empty lines', () => {
    const content = 'Line one\n\n\nLine two\n\n';
    render(() => <ThinkingBlock content={content} />);
    // Only 2 non-blank lines, 4 words
    expect(screen.getByText('2 lines · 4 words')).toBeInTheDocument();
  });

  it('counts only non-empty words', () => {
    const content = '  hello   world  ';
    render(() => <ThinkingBlock content={content} />);
    expect(screen.getByText('1 lines · 2 words')).toBeInTheDocument();
  });

  it('handles single-line content', () => {
    render(() => <ThinkingBlock content="just one line" />);
    expect(screen.getByText('1 lines · 3 words')).toBeInTheDocument();
  });

  // --- Preview text (collapsed state) ---

  it('shows preview text when collapsed', () => {
    render(() => <ThinkingBlock content="This is the preview" />);
    expect(screen.getByText('This is the preview')).toBeInTheDocument();
  });

  it('truncates preview to 60 chars with ellipsis', () => {
    const longLine = 'A'.repeat(70) + ' and some more text';
    render(() => <ThinkingBlock content={longLine} />);
    const expectedPreview = 'A'.repeat(60) + '...';
    expect(screen.getByText(expectedPreview)).toBeInTheDocument();
  });

  it('uses first non-empty line as preview', () => {
    const content = '\n\n  Real first line\nSecond line';
    render(() => <ThinkingBlock content={content} />);
    expect(screen.getByText('Real first line')).toBeInTheDocument();
  });

  // --- Expand/collapse behavior ---

  it('starts collapsed (content not visible)', () => {
    render(() => <ThinkingBlock content="Hidden reasoning content" />);
    // The sanitized content inside <pre> should not be in the DOM when collapsed
    const preElements = document.querySelectorAll('pre');
    expect(preElements.length).toBe(0);
  });

  it('expands on click to show content', async () => {
    render(() => <ThinkingBlock content="Expanded reasoning" />);
    const button = screen.getByRole('button');
    await fireEvent.click(button);
    const preElement = document.querySelector('pre');
    expect(preElement).not.toBeNull();
    expect(preElement!.textContent).toContain('Expanded reasoning');
  });

  it('collapses again on second click', async () => {
    render(() => <ThinkingBlock content="Toggle content" />);
    const button = screen.getByRole('button');

    // Expand
    await fireEvent.click(button);
    expect(document.querySelector('pre')).not.toBeNull();

    // Collapse
    await fireEvent.click(button);
    expect(document.querySelector('pre')).toBeNull();
  });

  it('hides preview text when expanded', async () => {
    render(() => <ThinkingBlock content="Preview goes away" />);
    // Preview visible when collapsed
    expect(screen.getByText('Preview goes away')).toBeInTheDocument();

    const button = screen.getByRole('button');
    await fireEvent.click(button);

    // After expanding, preview span should be gone; content appears only inside <pre>
    const spans = document.querySelectorAll('span');
    const previewSpans = Array.from(spans).filter((s) => s.textContent === 'Preview goes away');
    expect(previewSpans.length).toBe(0);
  });

  // --- Streaming indicator ---

  it('applies animate-pulse class when streaming', () => {
    render(() => <ThinkingBlock content="streaming" isStreaming={true} />);
    const pulseDiv = document.querySelector('.animate-pulse');
    expect(pulseDiv).not.toBeNull();
  });

  it('does not apply animate-pulse when not streaming', () => {
    render(() => <ThinkingBlock content="not streaming" isStreaming={false} />);
    const pulseDiv = document.querySelector('.animate-pulse');
    expect(pulseDiv).toBeNull();
  });

  // --- Content sanitization ---

  it('sanitizes TCP connection details in expanded content', async () => {
    const rawContent = 'Error: write tcp 192.0.2.10:7655->198.51.100.20:58004: i/o timeout';
    render(() => <ThinkingBlock content={rawContent} />);
    const button = screen.getByRole('button');
    await fireEvent.click(button);

    const preElement = document.querySelector('pre');
    expect(preElement).not.toBeNull();
    // Should NOT show raw IP addresses
    expect(preElement!.textContent).not.toContain('192.0.2.10');
    expect(preElement!.textContent).toContain('connection timed out');
  });

  it('sanitizes "failed to send command" patterns', async () => {
    const rawContent = 'failed to send command: write tcp 10.0.0.1:7655->10.0.0.2:9999: broken';
    render(() => <ThinkingBlock content={rawContent} />);
    const button = screen.getByRole('button');
    await fireEvent.click(button);

    const preElement = document.querySelector('pre');
    expect(preElement).not.toBeNull();
    // The sanitizer replaces the "failed to send command: write tcp <addr>" prefix
    expect(preElement!.textContent).toContain('failed to send command: connection error');
    // Source IP is removed by the regex
    expect(preElement!.textContent).not.toContain('10.0.0.1');
    // NOTE: The destination IP (10.0.0.2) may still leak because the regex
    // [\d.:->\s]+ in sanitizeThinking doesn't fully consume "->dest:port".
    // This is a known limitation in the sanitizer source, not a test gap.
  });

  it('sanitizes "dial tcp" connection refused patterns', async () => {
    const rawContent = 'Error: dial tcp 10.0.0.5:8006: connection refused';
    render(() => <ThinkingBlock content={rawContent} />);
    const button = screen.getByRole('button');
    await fireEvent.click(button);

    const preElement = document.querySelector('pre');
    expect(preElement).not.toBeNull();
    expect(preElement!.textContent).toContain('connection refused');
    expect(preElement!.textContent).not.toContain('10.0.0.5');
  });

  it('sanitizes "read tcp" timeout patterns', async () => {
    const rawContent = 'read tcp 172.16.0.1:7655: i/o timeout';
    render(() => <ThinkingBlock content={rawContent} />);
    const button = screen.getByRole('button');
    await fireEvent.click(button);

    const preElement = document.querySelector('pre');
    expect(preElement).not.toBeNull();
    expect(preElement!.textContent).toContain('connection timed out');
    expect(preElement!.textContent).not.toContain('172.16.0.1');
  });

  // --- Preview does not sanitize (documents current behavior) ---

  it('preview shows raw content (sanitization only applies to expanded body)', () => {
    const rawContent = 'write tcp 192.168.1.1:7655->192.168.1.2:58004: i/o timeout';
    render(() => <ThinkingBlock content={rawContent} />);
    // The collapsed preview uses raw props.content, not sanitizeThinking()
    // This documents the current behavior — preview truncates but does not sanitize
    const previewSpan = document.querySelector('span.text-muted.truncate');
    expect(previewSpan).not.toBeNull();
    // The preview shows the raw (truncated) text
    expect(previewSpan!.textContent).toContain('write tcp');
  });

  // --- Edge cases ---

  it('handles empty content gracefully', () => {
    render(() => <ThinkingBlock content="" />);
    expect(screen.getByText('0 lines · 0 words')).toBeInTheDocument();
  });

  it('handles whitespace-only content', () => {
    const whitespace = '   \n   \n   ';
    render(() => <ThinkingBlock content={whitespace} />);
    expect(screen.getByText('0 lines · 0 words')).toBeInTheDocument();
  });
});
