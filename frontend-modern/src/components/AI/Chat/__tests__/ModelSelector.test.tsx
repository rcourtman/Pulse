import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { ModelSelector } from '../ModelSelector';
import type { ModelInfo } from '../types';

afterEach(cleanup);

function makeModel(overrides?: Partial<ModelInfo>): ModelInfo {
  return {
    id: 'anthropic:claude-sonnet-4',
    name: 'Claude Sonnet 4',
    description: 'Fast and capable',
    notable: true,
    ...overrides,
  };
}

const SAMPLE_MODELS: ModelInfo[] = [
  makeModel({ id: 'anthropic:claude-sonnet-4', name: 'Claude Sonnet 4', notable: true }),
  makeModel({ id: 'anthropic:claude-opus-4', name: 'Claude Opus 4', notable: true }),
  makeModel({
    id: 'openai:gpt-4o',
    name: 'GPT-4o',
    description: 'OpenAI flagship',
    notable: true,
  }),
  makeModel({
    id: 'anthropic:claude-3-haiku',
    name: 'Claude 3 Haiku',
    description: 'Legacy model',
    notable: false,
  }),
  makeModel({
    id: 'ollama:llama3',
    name: 'Llama 3',
    description: 'Local model',
    notable: false,
  }),
];

describe('ModelSelector', () => {
  // --- Rendering & Selected Label ---

  it('renders the selected model label', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel="anthropic:claude-sonnet-4"
        onModelSelect={vi.fn()}
      />
    ));

    expect(screen.getByText('Claude Sonnet 4')).toBeInTheDocument();
  });

  it('shows "Default" when no model is selected', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    expect(screen.getByText('Default')).toBeInTheDocument();
  });

  it('shows "Default (label)" when defaultModelLabel is provided and no selection', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        defaultModelLabel="Claude Sonnet 4"
        onModelSelect={vi.fn()}
      />
    ));

    expect(screen.getByText('Default (Claude Sonnet 4)')).toBeInTheDocument();
  });

  it('shows raw model ID when selectedModel is not in the models list', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel="custom:my-model"
        onModelSelect={vi.fn()}
      />
    ));

    expect(screen.getByText('custom:my-model')).toBeInTheDocument();
  });

  it('shows the loading spinner when isLoading is true', () => {
    const { container } = render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        isLoading={true}
        onModelSelect={vi.fn()}
      />
    ));

    // The spinner is an animate-spin SVG
    const spinners = container.querySelectorAll('.animate-spin');
    expect(spinners.length).toBeGreaterThan(0);
  });

  // --- Dropdown Open/Close ---

  it('opens dropdown when button is clicked', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    // Dropdown not open initially
    expect(screen.queryByPlaceholderText('Search or enter model ID')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // Dropdown is now open — search input visible
    expect(screen.getByPlaceholderText('Search or enter model ID')).toBeInTheDocument();
  });

  it('closes dropdown when toggled again', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    const button = screen.getByTitle('Select model for this chat');
    fireEvent.click(button);
    expect(screen.getByPlaceholderText('Search or enter model ID')).toBeInTheDocument();

    fireEvent.click(button);
    expect(screen.queryByPlaceholderText('Search or enter model ID')).not.toBeInTheDocument();
  });

  // --- Default Option ---

  it('renders the Default option in dropdown', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.getByText('Use configured default model')).toBeInTheDocument();
  });

  it('renders default label with model name when defaultModelLabel provided', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        defaultModelLabel="GPT-4o"
        onModelSelect={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.getByText('Use configured default model (GPT-4o)')).toBeInTheDocument();
  });

  // --- Chat Override Option ---

  it('shows chat override option when chatOverrideModel is provided', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        chatOverrideModel="openai:gpt-4o"
        chatOverrideLabel="GPT-4o (override)"
        onModelSelect={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.getByText('Chat override')).toBeInTheDocument();
    expect(screen.getByText('GPT-4o (override)')).toBeInTheDocument();
  });

  it('uses chatOverrideModel as label when chatOverrideLabel is not provided', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        chatOverrideModel="openai:gpt-4o"
        onModelSelect={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // The override section label should contain the model ID
    const overrideButton = screen.getByText('Chat override').closest('button')!;
    expect(overrideButton.textContent).toContain('openai:gpt-4o');
  });

  it('does not show chat override option when chatOverrideModel is not provided', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.queryByText('Chat override')).not.toBeInTheDocument();
  });

  // --- Model Selection ---

  it('calls onModelSelect with empty string when Default is clicked', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="anthropic:claude-sonnet-4" onModelSelect={onModelSelect} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // Click the Default option (the one with "Use configured default model" text)
    const defaultButtons = screen.getAllByRole('button');
    const defaultOption = defaultButtons.find((btn) =>
      btn.textContent?.includes('Use configured default model')
    );
    expect(defaultOption).toBeDefined();
    fireEvent.click(defaultOption!);

    expect(onModelSelect).toHaveBeenCalledWith('');
  });

  it('calls onModelSelect with model ID when a model is clicked', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={onModelSelect} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    fireEvent.click(screen.getByText('GPT-4o'));

    expect(onModelSelect).toHaveBeenCalledWith('openai:gpt-4o');
  });

  it('closes dropdown after selection', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    expect(screen.getByPlaceholderText('Search or enter model ID')).toBeInTheDocument();

    fireEvent.click(screen.getByText('GPT-4o'));

    expect(screen.queryByPlaceholderText('Search or enter model ID')).not.toBeInTheDocument();
  });

  it('calls onModelSelect with chatOverrideModel when chat override is clicked', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        chatOverrideModel="openai:gpt-4o"
        chatOverrideLabel="Override"
        onModelSelect={onModelSelect}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    fireEvent.click(screen.getByText('Chat override'));

    expect(onModelSelect).toHaveBeenCalledWith('openai:gpt-4o');
  });

  // --- Notable Model Filtering ---

  it('shows only notable models by default', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // Notable models should be visible
    expect(screen.getByText('Claude Sonnet 4')).toBeInTheDocument();
    expect(screen.getByText('Claude Opus 4')).toBeInTheDocument();
    expect(screen.getByText('GPT-4o')).toBeInTheDocument();

    // Non-notable models should be hidden
    expect(screen.queryByText('Claude 3 Haiku')).not.toBeInTheDocument();
    expect(screen.queryByText('Llama 3')).not.toBeInTheDocument();
  });

  it('shows toggle with hidden model count', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // 2 non-notable models
    expect(screen.getByText('Show 2 older models')).toBeInTheDocument();
  });

  it('shows all models after clicking the toggle', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    fireEvent.click(screen.getByText('Show 2 older models'));

    // All models should now be visible
    expect(screen.getByText('Claude 3 Haiku')).toBeInTheDocument();
    expect(screen.getByText('Llama 3')).toBeInTheDocument();

    // Toggle should now say "Hide"
    expect(screen.getByText('Hide older models')).toBeInTheDocument();
  });

  it('falls back to showing all models when no models are notable', () => {
    const allNonNotable = SAMPLE_MODELS.map((m) => ({ ...m, notable: false }));
    render(() => (
      <ModelSelector models={allNonNotable} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // All models should be visible since none are notable
    expect(screen.getByText('Claude Sonnet 4')).toBeInTheDocument();
    expect(screen.getByText('Llama 3')).toBeInTheDocument();
  });

  // --- Search / Filtering ---

  it('filters models by name when searching', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'opus' } });

    expect(screen.getByText('Claude Opus 4')).toBeInTheDocument();
    // Other models filtered out
    expect(screen.queryByText('GPT-4o')).not.toBeInTheDocument();
  });

  it('searches all models including non-notable when query is present', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'llama' } });

    // Non-notable model should appear in search results
    expect(screen.getByText('Llama 3')).toBeInTheDocument();
  });

  it('searches by model ID', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'openai:gpt' } });

    expect(screen.getByText('GPT-4o')).toBeInTheDocument();
    expect(screen.queryByText('Claude Sonnet 4')).not.toBeInTheDocument();
  });

  it('searches by description', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'flagship' } });

    expect(screen.getByText('GPT-4o')).toBeInTheDocument();
  });

  it('searches by provider display name', () => {
    // Use "Google Gemini" as query — this only matches the PROVIDER_DISPLAY_NAMES mapping,
    // not the model ID or name, ensuring we're testing provider-name search specifically.
    const modelsWithGemini = [
      ...SAMPLE_MODELS,
      makeModel({
        id: 'gemini:gemini-2.0-flash',
        name: 'Gemini 2.0 Flash',
        description: 'Fast multimodal',
        notable: true,
      }),
    ];
    render(() => (
      <ModelSelector models={modelsWithGemini} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    // "Google" only matches via PROVIDER_DISPLAY_NAMES["gemini"] = "Google Gemini"
    fireEvent.input(searchInput, { target: { value: 'Google' } });

    expect(screen.getByText('Gemini 2.0 Flash')).toBeInTheDocument();
    expect(screen.queryByText('Claude Sonnet 4')).not.toBeInTheDocument();
    expect(screen.queryByText('GPT-4o')).not.toBeInTheDocument();
  });

  it('shows "No matching models" and no model rows when search has no results', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'zzz-nonexistent-zzz' } });

    expect(screen.getByText('No matching models.')).toBeInTheDocument();
    // Verify no model entries are rendered alongside the empty message
    expect(screen.queryByText('Claude Sonnet 4')).not.toBeInTheDocument();
    expect(screen.queryByText('GPT-4o')).not.toBeInTheDocument();
    expect(screen.queryByText('Llama 3')).not.toBeInTheDocument();
  });

  it('hides the show older models toggle when searching', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    expect(screen.getByText('Show 2 older models')).toBeInTheDocument();

    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'claude' } });

    expect(screen.queryByText(/older models/)).not.toBeInTheDocument();
  });

  // --- Custom Model ID ---

  it('shows custom model option when search query does not match any model ID', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'my-custom-model' } });

    expect(screen.getByText('Use "my-custom-model"')).toBeInTheDocument();
    expect(screen.getByText('Custom model ID')).toBeInTheDocument();
  });

  it('does not show custom model option when query exactly matches an existing model ID', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'anthropic:claude-sonnet-4' } });

    expect(screen.queryByText('Custom model ID')).not.toBeInTheDocument();
  });

  it('selects custom model when clicked', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={onModelSelect} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'my-custom-model' } });
    fireEvent.click(screen.getByText('Use "my-custom-model"'));

    expect(onModelSelect).toHaveBeenCalledWith('my-custom-model');
  });

  it('selects custom model on Enter key', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={onModelSelect} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'my-custom-model' } });
    fireEvent.keyDown(searchInput, { key: 'Enter' });

    expect(onModelSelect).toHaveBeenCalledWith('my-custom-model');
  });

  it('does not submit on non-Enter keys', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={onModelSelect} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'my-custom-model' } });
    fireEvent.keyDown(searchInput, { key: 'Escape' });

    expect(onModelSelect).not.toHaveBeenCalled();
  });

  // --- Error Display ---

  it('shows error message when error prop is provided', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        error="Failed to fetch models"
        onModelSelect={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.getByText('Failed to fetch models')).toBeInTheDocument();
  });

  it('does not show error section when no error', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.queryByText('Failed to fetch models')).not.toBeInTheDocument();
  });

  // --- Refresh Button ---

  it('shows refresh button when onRefresh is provided', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        onRefresh={vi.fn()}
        onModelSelect={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.getByTitle('Refresh models')).toBeInTheDocument();
  });

  it('does not show refresh button when onRefresh is not provided', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.queryByTitle('Refresh models')).not.toBeInTheDocument();
  });

  it('calls onRefresh when refresh button is clicked', () => {
    const onRefresh = vi.fn();
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        onRefresh={onRefresh}
        onModelSelect={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    fireEvent.click(screen.getByTitle('Refresh models'));

    expect(onRefresh).toHaveBeenCalledOnce();
  });

  it('disables refresh button when loading', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel=""
        isLoading={true}
        onRefresh={vi.fn()}
        onModelSelect={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    expect(screen.getByTitle('Refresh models')).toBeDisabled();
  });

  // --- Provider Grouping ---

  it('groups models by provider with headers', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // Should see provider headers (only notable models shown by default)
    expect(screen.getByText('Anthropic')).toBeInTheDocument();
    expect(screen.getByText('OpenAI')).toBeInTheDocument();
  });

  // --- Selected Model Highlighting ---

  it('highlights the currently selected model', () => {
    render(() => (
      <ModelSelector
        models={SAMPLE_MODELS}
        selectedModel="openai:gpt-4o"
        onModelSelect={vi.fn()}
      />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // GPT-4o appears twice: in the top button label and in the dropdown list.
    // Find the one inside the dropdown's model list (which has font-medium class in a flex container).
    const allGpt4o = screen.getAllByText('GPT-4o');
    const dropdownEntry = allGpt4o.find(
      (el) => el.closest('.max-h-72') !== null
    );
    expect(dropdownEntry).toBeDefined();
    const modelButton = dropdownEntry!.closest('button');
    expect(modelButton?.className).toContain('bg-purple-50');
  });

  // --- Model name display ---

  it('falls back to model ID segment when name is not provided', () => {
    const models = [makeModel({ id: 'anthropic:claude-test', name: '', notable: true })];
    render(() => (
      <ModelSelector models={models} selectedModel="anthropic:claude-test" onModelSelect={vi.fn()} />
    ));

    // selectedLabel should use id.split(':').pop() when name is empty but match exists
    // Actually, empty name returns falsy, so it falls to id.split(':').pop()
    expect(screen.getByText('claude-test')).toBeInTheDocument();
  });

  // --- Empty models list ---

  it('handles empty models list gracefully', () => {
    render(() => (
      <ModelSelector models={[]} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));

    // Should still show Default option and no crash
    expect(screen.getByText('Use configured default model')).toBeInTheDocument();
  });

  // --- Search resets on close ---

  it('resets search query after selection', () => {
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: 'opus' } });

    // Select a model
    fireEvent.click(screen.getByText('Claude Opus 4'));

    // Re-open dropdown
    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const newSearchInput = screen.getByPlaceholderText('Search or enter model ID') as HTMLInputElement;
    expect(newSearchInput.value).toBe('');
  });

  // --- Click outside ---

  it('closes dropdown when clicking outside the dropdown container', () => {
    render(() => (
      <div>
        <span data-testid="outside">Outside</span>
        <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={vi.fn()} />
      </div>
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    expect(screen.getByPlaceholderText('Search or enter model ID')).toBeInTheDocument();

    // Simulate click outside the dropdown container
    fireEvent.click(screen.getByTestId('outside'));

    expect(screen.queryByPlaceholderText('Search or enter model ID')).not.toBeInTheDocument();
  });

  // --- Enter key with whitespace trimming ---

  it('trims whitespace when submitting custom model via Enter', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={onModelSelect} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.input(searchInput, { target: { value: '  my-model  ' } });
    fireEvent.keyDown(searchInput, { key: 'Enter' });

    // customModelCandidate trims the search query
    expect(onModelSelect).toHaveBeenCalledWith('my-model');
  });

  it('does not submit on Enter when search query is empty', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={onModelSelect} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    fireEvent.keyDown(searchInput, { key: 'Enter' });

    expect(onModelSelect).not.toHaveBeenCalled();
  });

  it('submits existing model ID on Enter (selects by candidate, not custom)', () => {
    const onModelSelect = vi.fn();
    render(() => (
      <ModelSelector models={SAMPLE_MODELS} selectedModel="" onModelSelect={onModelSelect} />
    ));

    fireEvent.click(screen.getByTitle('Select model for this chat'));
    const searchInput = screen.getByPlaceholderText('Search or enter model ID');
    // Type an exact existing model ID — custom option hidden, but Enter still submits
    fireEvent.input(searchInput, { target: { value: 'anthropic:claude-sonnet-4' } });
    fireEvent.keyDown(searchInput, { key: 'Enter' });

    expect(onModelSelect).toHaveBeenCalledWith('anthropic:claude-sonnet-4');
  });
});
