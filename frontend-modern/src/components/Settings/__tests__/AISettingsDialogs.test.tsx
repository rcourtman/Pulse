import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import type { AIProvider } from '@/types/ai';
import { AISettingsDialogs } from '../AISettingsDialogs';
import type { PendingRemoveProvider } from '../useAISettingsState';

describe('AISettingsDialogs', () => {
  afterEach(() => {
    cleanup();
  });

  const renderSetupDialog = (initialProvider: AIProvider = 'anthropic') => {
    const [showSetupModal, setShowSetupModal] = createSignal(true);
    const [setupProvider, setSetupProvider] = createSignal<AIProvider>(initialProvider);
    const [setupApiKey, setSetupApiKey] = createSignal('');
    const [setupOpenAIBaseUrl, setSetupOpenAIBaseUrl] = createSignal('');
    const [setupOllamaUrl, setSetupOllamaUrl] = createSignal('http://localhost:11434');
    const [setupSaving] = createSignal(false);
    const handleCloseSetupModal = vi.fn(() => setShowSetupModal(false));
    const handleSetupSubmit = vi.fn(async () => {});

    render(() => (
      <AISettingsDialogs
        showSetupModal={showSetupModal}
        setupProvider={setupProvider}
        setSetupProvider={setSetupProvider}
        setupApiKey={setupApiKey}
        setSetupApiKey={setSetupApiKey}
        setupOpenAIBaseUrl={setupOpenAIBaseUrl}
        setSetupOpenAIBaseUrl={setSetupOpenAIBaseUrl}
        setupOllamaUrl={setupOllamaUrl}
        setSetupOllamaUrl={setSetupOllamaUrl}
        setupSaving={setupSaving}
        handleCloseSetupModal={handleCloseSetupModal}
        handleSetupSubmit={handleSetupSubmit}
      />
    ));

    return { handleCloseSetupModal, handleSetupSubmit, setSetupProvider };
  };

  it('keeps the provider list inside a bounded scrollable body with pinned footer controls', () => {
    renderSetupDialog();

    const dialog = screen.getByRole('dialog', { name: 'Set up Pulse Intelligence' });
    const body = within(dialog).getByTestId('ai-setup-dialog-body');
    expect(body.className).toContain('overflow-y-auto');
    expect(body.className).toContain('min-h-0');
    expect(body.className).toContain('flex-1');

    const submitButton = within(dialog).getByRole('button', { name: 'Enable Pulse Intelligence' });
    expect(body.contains(submitButton)).toBe(false);
    expect(submitButton).toBeDisabled();
  });

  it('allows keyless OpenAI-compatible setup via Custom Base URL', () => {
    const { handleSetupSubmit } = renderSetupDialog('openai');

    const dialog = screen.getByRole('dialog', { name: 'Set up Pulse Intelligence' });
    const submitButton = within(dialog).getByRole('button', { name: 'Enable Pulse Intelligence' });
    expect(submitButton).toBeDisabled();

    const baseUrlInput = within(dialog).getByLabelText('Custom Base URL');
    fireEvent.input(baseUrlInput, { target: { value: 'http://127.0.0.1:1234/v1' } });
    expect(submitButton).toBeEnabled();

    fireEvent.click(submitButton);
    expect(handleSetupSubmit).toHaveBeenCalledTimes(1);
  });

  it('uses provider-specific docs link copy for docs-only providers', () => {
    renderSetupDialog('zai');

    const dialog = screen.getByRole('dialog', { name: 'Set up Pulse Intelligence' });
    expect(within(dialog).getByRole('link', { name: 'Open docs →' })).toBeInTheDocument();
    expect(within(dialog).queryByText('Get your API key →')).not.toBeInTheDocument();
  });

  it('renders an accessible confirmation dialog for removing a provider', () => {
    const [showSetupModal] = createSignal(false);
    const [setupProvider, setSetupProvider] = createSignal<AIProvider>('anthropic');
    const [setupApiKey, setSetupApiKey] = createSignal('');
    const [setupOpenAIBaseUrl, setSetupOpenAIBaseUrl] = createSignal('');
    const [setupOllamaUrl, setSetupOllamaUrl] = createSignal('http://localhost:11434');
    const [setupSaving] = createSignal(false);
    const [pendingRemove, setPendingRemove] = createSignal<PendingRemoveProvider | null>({
      provider: 'openai',
      providerLabel: 'OpenAI',
      message: 'Warning: this is your only configured provider.',
    });
    const handleCancel = vi.fn(() => setPendingRemove(null));
    const handleConfirm = vi.fn(async () => {});

    render(() => (
      <AISettingsDialogs
        showSetupModal={showSetupModal}
        setupProvider={setupProvider}
        setSetupProvider={setSetupProvider}
        setupApiKey={setupApiKey}
        setSetupApiKey={setSetupApiKey}
        setupOpenAIBaseUrl={setupOpenAIBaseUrl}
        setSetupOpenAIBaseUrl={setSetupOpenAIBaseUrl}
        setupOllamaUrl={setupOllamaUrl}
        setSetupOllamaUrl={setSetupOllamaUrl}
        setupSaving={setupSaving}
        handleCloseSetupModal={vi.fn()}
        handleSetupSubmit={vi.fn(async () => {})}
        pendingRemoveProvider={pendingRemove}
        removeSaving={setupSaving}
        handleCancelRemoveProvider={handleCancel}
        handleConfirmRemoveProvider={handleConfirm}
      />
    ));

    const dialog = screen.getByRole('dialog', { name: 'Remove OpenAI' });
    expect(within(dialog).getByText(/only configured provider/)).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole('button', { name: 'Remove provider' }));
    expect(handleConfirm).toHaveBeenCalledTimes(1);

    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(handleCancel).toHaveBeenCalledTimes(1);
  });
});
