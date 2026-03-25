import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SelectionCardGroup } from './SelectionCardGroup';
import selectionCardGroupSource from './SelectionCardGroup.tsx?raw';
import selectionCardGroupModelSource from './selectionCardGroupModel.ts?raw';
import selectionCardGroupStateSource from './useSelectionCardGroupState.ts?raw';

describe('SelectionCardGroup', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps shell, runtime, and model owners split', () => {
    expect(selectionCardGroupSource).toContain('useSelectionCardGroupState');
    expect(selectionCardGroupSource).toContain('getSelectionCardGroupClass');
    expect(selectionCardGroupSource).toContain('getSelectionCardButtonClass');
    expect(selectionCardGroupSource).toContain('getSelectionCardTitleClass');
    expect(selectionCardGroupSource).not.toContain('resolveSelectionCardTone');
    expect(selectionCardGroupSource).not.toContain('props.onChange(option.value)');
    expect(selectionCardGroupSource).not.toContain('groupClassByVariant');

    expect(selectionCardGroupStateSource).toContain('export function useSelectionCardGroupState');
    expect(selectionCardGroupStateSource).toContain('createMemo');
    expect(selectionCardGroupStateSource).toContain('resolveSelectionCardTone');
    expect(selectionCardGroupStateSource).toContain('props.disabled || option.disabled');
    expect(selectionCardGroupStateSource).toContain('props.onChange(option.value)');

    expect(selectionCardGroupModelSource).toContain('resolveSelectionCardGroupVariant');
    expect(selectionCardGroupModelSource).toContain('resolveSelectionCardTone');
    expect(selectionCardGroupModelSource).toContain('getSelectionCardButtonClass');
    expect(selectionCardGroupModelSource).toContain('getSelectionCardTitleClass');
    expect(selectionCardGroupModelSource).toContain("compact: 'grid grid-cols-2 gap-2'");
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

  it('blocks disabled selection changes in the runtime owner', () => {
    const onChange = vi.fn();

    render(() => (
      <SelectionCardGroup
        options={[
          { value: 'stable', title: 'Stable' },
          { value: 'rc', title: 'Pre-release', disabled: true },
        ]}
        value="stable"
        onChange={onChange}
        variant="detail"
      />
    ));

    const rcButton = screen.getByRole('button', { name: /pre-release/i });
    expect(rcButton).toBeDisabled();

    fireEvent.click(rcButton);
    expect(onChange).not.toHaveBeenCalled();
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
            title: 'Pre-release',
            description: 'Early preview builds',
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
    const rcButton = screen.getByRole('button', { name: /pre-release early preview builds/i });

    expect(stableButton.className).toContain('border-green-500');
    expect(stableButton.className).toContain('bg-green-50');
    expect(rcButton.className).toContain('border-border');
  });
});
