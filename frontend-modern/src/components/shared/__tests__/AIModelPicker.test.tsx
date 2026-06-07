import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
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
    provider: 'openrouter',
  },
  {
    id: 'openrouter:legacy/model-v1',
    name: 'Legacy Model V1',
    description: 'Older provider catalog entry',
    notable: false,
    provider: 'openrouter',
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

    expect(screen.getAllByText('MiniMax: MiniMax M2.5 via OpenRouter').length).toBeGreaterThan(0);
    expect(screen.getByText('GPT-5.1 Mini')).toBeInTheDocument();
    expect(screen.queryByText('Legacy Model V1 via OpenRouter')).not.toBeInTheDocument();
    expect(screen.getByText('Show 1 older models')).toBeInTheDocument();
  });

  it('labels gateway-routed selected models with the transport provider', () => {
    const routeModels: ModelInfo[] = [
      {
        id: 'openrouter:deepseek/deepseek-v4-pro',
        name: 'DeepSeek: DeepSeek V4 Pro',
        provider: 'openrouter',
        notable: true,
      },
      {
        id: 'deepseek:deepseek-v4-pro',
        name: 'DeepSeek: DeepSeek V4 Pro',
        provider: 'deepseek',
        notable: true,
      },
    ];

    render(() => (
      <AIModelPicker
        models={routeModels}
        selectedModel="openrouter:deepseek/deepseek-v4-pro"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    expect(screen.getByText('DeepSeek: DeepSeek V4 Pro via OpenRouter')).toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Select shared default model'));

    expect(screen.getAllByText('DeepSeek: DeepSeek V4 Pro via OpenRouter').length).toBeGreaterThan(
      0,
    );
    expect(screen.getByText('DeepSeek: DeepSeek V4 Pro')).toBeInTheDocument();
  });

  it('labels Pulse-owned mock routes as Assistant runtime choices', () => {
    const routeModels: ModelInfo[] = [
      {
        id: 'pulse:mock-assistant',
        name: 'Pulse mock Assistant',
        provider: 'pulse',
        notable: true,
      },
    ];

    render(() => (
      <AIModelPicker
        models={routeModels}
        selectedModel="pulse:mock-assistant"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    expect(screen.getByText('Pulse mock Assistant')).toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Select shared default model'));

    expect(screen.getByText('Pulse')).toBeInTheDocument();
    expect(screen.getAllByText('Pulse mock Assistant').length).toBeGreaterThan(0);
    expect(screen.queryByText('pulse:mock-assistant')).not.toBeInTheDocument();
  });

  it('separates selected model labels from selection badges in composer chrome', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel=""
        onModelSelect={vi.fn()}
        emptySelectionLabel="Qwen: Qwen3.7 Plus via OpenRouter"
        selectionBadge="default"
        title="Select shared default model"
      />
    ));

    const button = screen.getByTitle('Select shared default model');
    expect(button.textContent).toContain('Qwen: Qwen3.7 Plus via OpenRouter · default');
    expect(button.textContent).not.toContain('OpenRouterdefault');
    expect(screen.getByRole('button', { name: 'Qwen: Qwen3.7 Plus via OpenRouter, default' })).toBe(
      button,
    );
  });

  it('marks the selected catalog row as current in the model list', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:minimax/minimax-m2.5"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    const currentOption = screen.getByRole('option', {
      name: /MiniMax: MiniMax M2\.5 via OpenRouter.*Current/,
    });
    expect(currentOption).toHaveAttribute('aria-selected', 'true');
    expect(currentOption).toHaveAttribute(
      'aria-label',
      'MiniMax: MiniMax M2.5 via OpenRouter, Current. Current OpenRouter model. openrouter:minimax/minimax-m2.5',
    );
    expect(currentOption).toHaveTextContent('Current');
  });

  it('marks the inherited default option as current when no chat route is selected', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel=""
        onModelSelect={vi.fn()}
        defaultOption={{
          label: 'Default',
          description: 'Use configured default model',
        }}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    const currentOption = screen.getByRole('option', { name: /Default.*Current/ });
    expect(currentOption).toHaveAttribute('aria-selected', 'true');
    expect(currentOption).toHaveAttribute(
      'aria-label',
      'Default, Current. Use configured default model',
    );
    expect(currentOption).toHaveTextContent('Use configured default model');
  });

  it('labels the dropdown and listbox with the picker title', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:minimax/minimax-m2.5"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    const button = screen.getByTitle('Select shared default model');
    fireEvent.click(button);

    expect(button.getAttribute('aria-controls')?.endsWith('-listbox')).toBe(true);
    expect(screen.getByRole('dialog', { name: 'Select shared default model' })).toBeInTheDocument();
    expect(
      screen.getByRole('listbox', { name: 'Select shared default model' }),
    ).toBeInTheDocument();
  });

  it('moves keyboard focus from search through model options', async () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:minimax/minimax-m2.5"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    const currentOption = screen.getByRole('option', {
      name: 'MiniMax: MiniMax M2.5 via OpenRouter, Current. Current OpenRouter model. openrouter:minimax/minimax-m2.5',
    });
    const nextOption = screen.getByRole('option', {
      name: 'GPT-5.1 Mini. openai:gpt-5.1-mini',
    });
    const showOlderOption = screen.getByRole('option', {
      name: 'Show 1 older models',
    });

    await waitFor(() => {
      expect(document.activeElement).toBe(searchInput);
    });

    fireEvent.keyDown(searchInput, { key: 'ArrowDown' });
    expect(document.activeElement).toBe(currentOption);

    fireEvent.keyDown(currentOption, { key: 'ArrowDown' });
    expect(document.activeElement).toBe(nextOption);

    fireEvent.keyDown(nextOption, { key: 'ArrowUp' });
    expect(document.activeElement).toBe(currentOption);

    fireEvent.keyDown(currentOption, { key: 'End' });
    expect(document.activeElement).toBe(showOlderOption);

    fireEvent.keyDown(showOlderOption, { key: 'ArrowUp' });
    expect(document.activeElement).toBe(nextOption);

    fireEvent.keyDown(nextOption, { key: 'Home' });
    expect(document.activeElement).toBe(currentOption);
  });

  it('activates focused options from the keyboard', async () => {
    const onModelSelect = vi.fn();
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:minimax/minimax-m2.5"
        onModelSelect={onModelSelect}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    const currentOption = screen.getByRole('option', {
      name: 'MiniMax: MiniMax M2.5 via OpenRouter, Current. Current OpenRouter model. openrouter:minimax/minimax-m2.5',
    });

    await waitFor(() => {
      expect(document.activeElement).toBe(searchInput);
    });

    fireEvent.keyDown(searchInput, { key: 'ArrowDown' });
    fireEvent.keyDown(currentOption, { key: 'Enter' });

    expect(onModelSelect).toHaveBeenCalledWith('openrouter:minimax/minimax-m2.5');
  });

  it('reveals older catalog rows from the focused disclosure option', async () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:minimax/minimax-m2.5"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    const showOlderOption = screen.getByRole('option', {
      name: 'Show 1 older models',
    });

    await waitFor(() => {
      expect(document.activeElement).toBe(searchInput);
    });

    fireEvent.keyDown(searchInput, { key: 'End' });
    expect(document.activeElement).toBe(showOlderOption);

    fireEvent.keyDown(showOlderOption, { key: 'Enter' });

    expect(screen.getByRole('option', { name: 'Hide older models' })).toBeInTheDocument();
    expect(screen.getByText('Legacy Model V1 via OpenRouter')).toBeInTheDocument();
  });

  it('supports page, home, end, and Escape from the search field', async () => {
    const pageModels: ModelInfo[] = Array.from({ length: 12 }, (_, index) => ({
      id: `openrouter:page/model-${index}`,
      name: `Page Model ${index}`,
      notable: true,
      provider: 'openrouter',
    }));
    const onParentKeyDown = vi.fn();

    render(() => (
      <div onKeyDown={onParentKeyDown}>
        <AIModelPicker
          models={pageModels}
          selectedModel="openrouter:page/model-0"
          onModelSelect={vi.fn()}
          title="Select shared default model"
        />
      </div>
    ));

    const button = screen.getByTitle('Select shared default model');
    fireEvent.click(button);

    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    const optionFor = (index: number) =>
      screen.getByRole('option', {
        name: new RegExp(`^Page Model ${index} via OpenRouter(?:, Current)?\\. `),
      });

    await waitFor(() => {
      expect(document.activeElement).toBe(searchInput);
    });

    fireEvent.keyDown(searchInput, { key: 'PageDown' });
    expect(document.activeElement).toBe(optionFor(10));

    searchInput.focus();
    fireEvent.keyDown(searchInput, { key: 'PageUp' });
    expect(document.activeElement).toBe(optionFor(11));

    searchInput.focus();
    fireEvent.keyDown(searchInput, { key: 'End' });
    expect(document.activeElement).toBe(optionFor(11));

    searchInput.focus();
    fireEvent.keyDown(searchInput, { key: 'Home' });
    expect(document.activeElement).toBe(optionFor(0));

    searchInput.focus();
    fireEvent.keyDown(searchInput, { key: 'Escape' });
    expect(
      screen.queryByRole('listbox', { name: 'Select shared default model' }),
    ).not.toBeInTheDocument();
    await waitFor(() => {
      expect(document.activeElement).toBe(button);
    });
    expect(onParentKeyDown).not.toHaveBeenCalled();
  });

  it('moves keyboard focus to the first filtered result while searching', async () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel="openrouter:minimax/minimax-m2.5"
        onModelSelect={vi.fn()}
        title="Select shared default model"
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'legacy' } });
    const legacyOption = screen.getByRole('option', {
      name: 'Legacy Model V1 via OpenRouter. Older provider catalog entry. openrouter:legacy/model-v1',
    });

    await waitFor(() => {
      expect(document.activeElement).toBe(searchInput);
    });

    fireEvent.keyDown(searchInput, { key: 'ArrowDown' });
    expect(document.activeElement).toBe(legacyOption);
  });

  it('closes the picker and returns focus to the trigger from option Escape', async () => {
    const onParentKeyDown = vi.fn();

    render(() => (
      <div onKeyDown={onParentKeyDown}>
        <AIModelPicker
          models={models}
          selectedModel="openrouter:minimax/minimax-m2.5"
          onModelSelect={vi.fn()}
          title="Select shared default model"
        />
      </div>
    ));

    const button = screen.getByTitle('Select shared default model');
    fireEvent.click(button);

    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    const currentOption = screen.getByRole('option', {
      name: 'MiniMax: MiniMax M2.5 via OpenRouter, Current. Current OpenRouter model. openrouter:minimax/minimax-m2.5',
    });
    await waitFor(() => {
      expect(document.activeElement).toBe(searchInput);
    });
    fireEvent.keyDown(searchInput, { key: 'ArrowDown' });

    expect(document.activeElement).toBe(currentOption);
    fireEvent.keyDown(currentOption, { key: 'Escape' });

    expect(screen.queryByPlaceholderText('Search or enter model ID')).not.toBeInTheDocument();
    await waitFor(() => {
      expect(document.activeElement).toBe(button);
    });
    expect(onParentKeyDown).not.toHaveBeenCalled();
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

  it('opens the dropdown above the button when composer chrome leaves no room below', () => {
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(390);
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(700);
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
      bottom: 668,
      height: 40,
      left: 16,
      right: 332,
      top: 628,
      width: 316,
      x: 16,
      y: 628,
      toJSON: () => ({}),
    } as DOMRect);

    fireEvent.click(button);

    const dropdown = document.querySelector('[data-ai-model-picker] .fixed') as HTMLElement;
    expect(dropdown.style.top).toBe('');
    expect(dropdown.style.bottom).toBe('76px');
    expect(dropdown.style.maxHeight).toBe('384px');
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

    expect(screen.getByText('Legacy Model V1 via OpenRouter')).toBeInTheDocument();
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

    expect(screen.getByText('MiniMax: MiniMax M2.5 via OpenRouter')).toBeInTheDocument();
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

    expect(screen.getAllByText('Legacy Model V1 via OpenRouter').length).toBeGreaterThan(0);
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

  it('does not offer malformed explicit custom routes', () => {
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
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');

    for (const value of [
      'openrouter:',
      ':custom/model',
      'https://openrouter.ai/models/deepseek/deepseek-chat',
      'openrouter:/custom/model',
      'open router:custom/model',
    ]) {
      fireEvent.input(searchInput, { target: { value } });
      expect(screen.queryByText(`Use "${value}"`)).not.toBeInTheDocument();
      fireEvent.keyDown(searchInput, { key: 'Enter' });
      expect(onModelSelect).not.toHaveBeenCalled();
    }
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

  it('renders priority model sections above provider groups and removes duplicate rows', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel=""
        onModelSelect={vi.fn()}
        title="Select shared default model"
        modelSections={[
          {
            title: 'Recent',
            modelIds: ['openrouter:minimax/minimax-m2.5', 'openrouter:custom/model'],
          },
        ]}
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    expect(screen.getByText('Recent')).toBeInTheDocument();
    expect(screen.getByText('Custom: Model via OpenRouter')).toBeInTheDocument();
    expect(screen.getByText('Recent custom model route')).toBeInTheDocument();
    expect(screen.getAllByText('MiniMax: MiniMax M2.5 via OpenRouter')).toHaveLength(1);
  });

  it('drops malformed custom routes from priority model sections', () => {
    render(() => (
      <AIModelPicker
        models={models}
        selectedModel=""
        onModelSelect={vi.fn()}
        title="Select shared default model"
        modelSections={[
          {
            title: 'Recent',
            modelIds: [
              'openrouter:',
              'https://openrouter.ai/models/foo',
              'openrouter:custom/model',
            ],
          },
        ]}
      />
    ));

    fireEvent.click(screen.getByTitle('Select shared default model'));

    expect(screen.getByText('Custom: Model via OpenRouter')).toBeInTheDocument();
    expect(screen.queryByText('Openrouter:')).not.toBeInTheDocument();
    expect(screen.queryByText(/openrouter\.ai/)).not.toBeInTheDocument();
  });
});
