import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AIModelPicker } from '@/components/shared/AIModelPicker';
import type { ModelInfo } from '@/types/ai';

afterEach(() => {
  vi.restoreAllMocks();
  cleanup();
});

const models: ModelInfo[] = [
  {
    id: 'openrouter:minimax/minimax-m2.5',
    name: 'MiniMax: MiniMax M2.5',
    description: 'Current OpenRouter model',
    notable: true,
  },
  {
    id: 'openrouter:legacy/model-v1',
    name: 'Legacy Model V1',
    description: 'Older provider catalog entry',
    notable: false,
  },
  {
    id: 'openai:gpt-5.1-mini',
    name: 'GPT-5.1 Mini',
    notable: true,
  },
];

describe('AIModelPicker', () => {
  it('keeps older provider catalog entries behind the disclosure by default', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:minimax/minimax-m2.5"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    expect(screen.getAllByText('MiniMax: MiniMax M2.5').length).toBeGreaterThan(0);
    expect(screen.getByText('GPT-5.1 Mini')).toBeInTheDocument();
    expect(screen.queryByText('Legacy Model V1')).not.toBeInTheDocument();
    expect(screen.getByText('Show 1 older models')).toBeInTheDocument();
  });

  it('constrains the dropdown to the available mobile viewport height', () => {
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(760);
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(850);
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:minimax/minimax-m2.5"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    const button = screen.getByTitle('Select shared default model');
    vi.spyOn(button, 'getBoundingClientRect').mockReturnValue({
      bottom: 470,
      height: 40,
      left: 24,
      right: 624,
      top: 430,
      width: 600,
      x: 24,
      y: 430,
      toJSON: () => ({}),
    } as DOMRect);

    fireEvent.click(button);

    const dropdown = document.querySelector('[data-ai-model-picker] .fixed') as HTMLElement;
    expect(dropdown.style.maxHeight).toBe('292px');
    expect(screen.getByRole('listbox').style.maxHeight).toBe('240px');
  });

  it('searches the full catalog without requiring the older-model disclosure first', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel=""
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));
    fireEvent.input(screen.getByPlaceholderText('Search or enter model ID'), {
      target: { value: 'legacy' },
    });

    expect(screen.getByText('Legacy Model V1')).toBeInTheDocument();
    expect(screen.queryByText('Show 1 older models')).not.toBeInTheDocument();
  });

  it('does not offer a custom model for plain name searches', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel=""
        onModelSelect={onModelSelect}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));
    fireEvent.input(screen.getByPlaceholderText('Search or enter model ID'), {
      target: { value: 'minimax' },
    });

    expect(screen.getByText('MiniMax: MiniMax M2.5')).toBeInTheDocument();
    expect(screen.queryByText('Use "minimax"')).not.toBeInTheDocument();

    fireEvent.keyDown(screen.getByPlaceholderText('Search or enter model ID'), { key: 'Enter' });

    expect(onModelSelect).not.toHaveBeenCalled();
  });

  it('keeps a non-notable current selection visible before expanding older models', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:legacy/model-v1"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    expect(screen.getAllByText('Legacy Model V1').length).toBeGreaterThan(0);
    expect(screen.queryByText('Show 1 older models')).not.toBeInTheDocument();
  });

  it('selects a custom model ID from search', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel=""
        onModelSelect={onModelSelect}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));
    fireEvent.input(screen.getByPlaceholderText('Search or enter model ID'), {
      target: { value: 'openrouter:custom/model' },
    });
    fireEvent.click(screen.getByText('Use "openrouter:custom/model"'));

    expect(onModelSelect).toHaveBeenCalledWith('openrouter:custom/model');
  });

  it('selects an exact model ID on Enter', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel=""
        onModelSelect={onModelSelect}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));
    fireEvent.input(screen.getByPlaceholderText('Search or enter model ID'), {
      target: { value: 'openrouter:minimax/minimax-m2.5' },
    });
    fireEvent.keyDown(screen.getByPlaceholderText('Search or enter model ID'), { key: 'Enter' });

    expect(onModelSelect).toHaveBeenCalledWith('openrouter:minimax/minimax-m2.5');
  });
});
