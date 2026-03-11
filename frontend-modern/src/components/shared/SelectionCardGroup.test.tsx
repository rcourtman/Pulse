import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SelectionCardGroup } from './SelectionCardGroup';

describe('SelectionCardGroup', () => {
  afterEach(() => {
    cleanup();
  });

  it('routes compact card selection changes through the shared primitive', () => {
    const onChange = vi.fn();

    render(() => (
      <SelectionCardGroup
        options={[
          { value: 'anthropic', title: 'Anthropic', description: 'Claude' },
          { value: 'openai', title: 'OpenAI', description: 'ChatGPT' },
        ]}
        value="anthropic"
        onChange={onChange}
        variant="compact"
      />
    ));

    const anthropicButton = screen.getByRole('button', { name: /anthropic claude/i });
    const openAIButton = screen.getByRole('button', { name: /openai chatgpt/i });

    expect(anthropicButton).toHaveAttribute('aria-pressed', 'true');
    expect(openAIButton).toHaveAttribute('aria-pressed', 'false');

    fireEvent.click(openAIButton);
    expect(onChange).toHaveBeenCalledWith('openai');
  });

  it('supports detail cards with success tone styling', () => {
    render(() => (
      <SelectionCardGroup
        options={[
          {
            value: 'stable',
            title: 'Stable',
            description: 'Production-ready releases',
            tone: 'success',
          },
          {
            value: 'rc',
            title: 'Release Candidate',
            description: 'Preview upcoming features',
            tone: 'accent',
          },
        ]}
        value="stable"
        onChange={() => undefined}
        variant="detail"
      />
    ));

    const stableButton = screen.getByRole('button', {
      name: /stable production-ready releases/i,
    });
    const rcButton = screen.getByRole('button', { name: /release candidate preview upcoming features/i });

    expect(stableButton.className).toContain('border-green-500');
    expect(stableButton.className).toContain('bg-green-50');
    expect(rcButton.className).toContain('border-border');
  });
});
