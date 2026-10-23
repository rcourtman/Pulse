import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { AISettings, PatrolCostProjection, PatrolModelReadinessSnapshot } from '@/types/ai';
import { PatrolSetupPanel } from '../PatrolSetupPanel';

const mocks = vi.hoisted(() => ({
  updateSettings: vi.fn(),
  getModels: vi.fn(),
  testProvider: vi.fn(),
  getPatrolCostPreview: vi.fn(),
  getPatrolModelGuidance: vi.fn(),
  runPatrolModelReadiness: vi.fn(),
  syncAIRuntimeSettings: vi.fn(),
  refreshEnabledFromServer: vi.fn(),
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    updateSettings: mocks.updateSettings,
    getModels: mocks.getModels,
    testProvider: mocks.testProvider,
  },
}));
vi.mock('@/api/aiPatrolCost', () => ({
  getPatrolCostPreview: mocks.getPatrolCostPreview,
  getPatrolModelGuidance: mocks.getPatrolModelGuidance,
}));
vi.mock('@/api/patrol', () => ({ runPatrolModelReadiness: mocks.runPatrolModelReadiness }));
vi.mock('@/stores/aiRuntimeState', () => ({ syncAIRuntimeSettings: mocks.syncAIRuntimeSettings }));
vi.mock('@/stores/aiAvailability', () => ({
  aiAvailabilityStore: {
    refreshEnabledFromServer: mocks.refreshEnabledFromServer,
  },
}));

const haikuProjection = {
  provider: 'anthropic',
  model: 'claude-haiku-4-5',
  model_route: 'anthropic:claude-haiku-4-5',
  billed_per_token: true,
  pricing_known: true,
  interval_minutes: 360,
  projected_30d_usd: 15.24,
  recommended_interval_minutes: 720,
  interval_estimates: [
    { interval_minutes: 360, projected_30d_usd: 15.24 },
    { interval_minutes: 720, projected_30d_usd: 7.62 },
  ],
} as PatrolCostProjection;

const settings = (overrides: Partial<AISettings> = {}) =>
  ({ enabled: true, model: '', patrol_interval_minutes: 360, ...overrides }) as AISettings;

const renderPanel = (readinessProvider?: string) => {
  const props = { onStart: vi.fn(), onDone: vi.fn(), onRunPatrol: vi.fn() };
  render(() => <PatrolSetupPanel readinessProvider={readinessProvider} {...props} />);
  return props;
};

describe('PatrolSetupPanel', () => {
  beforeEach(() => {
    mocks.getPatrolModelGuidance.mockResolvedValue({
      rules: [
        { provider: 'anthropic', model_prefix: 'claude-haiku-4-5', level: 'suggested', reason: '' },
        {
          provider: 'ollama',
          model_prefix: 'qwen3:8b',
          model_exact: true,
          level: 'recommended',
          reason: '',
        },
      ],
    });
    mocks.getPatrolCostPreview.mockResolvedValue(haikuProjection);
    mocks.testProvider.mockResolvedValue({ success: true, message: 'ok', provider: 'anthropic' });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('names the model and its monthly cost before anything is saved', async () => {
    renderPanel();
    expect(await screen.findByText('claude-haiku-4-5')).toBeInTheDocument();
    expect(
      await screen.findByText(
        /About \$7\.62 a month on your provider bill, checking every 12 hours/,
      ),
    ).toBeInTheDocument();
    expect(mocks.getPatrolCostPreview).toHaveBeenCalledWith({
      model: 'anthropic:claude-haiku-4-5',
    });
    expect(screen.getByRole('button', { name: 'Connect and check' })).toBeDisabled();
    expect(mocks.updateSettings).not.toHaveBeenCalled();
  });

  it('starts on the provider Patrol readiness names, such as the one whose key was refused', async () => {
    renderPanel('gemini');
    expect(await screen.findByRole('button', { name: /Gemini/ })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByLabelText('Google Gemini API key')).toBeInTheDocument();
  });

  it('connects, picks and saves the guided model, checks it, and offers the first run', async () => {
    mocks.updateSettings
      .mockResolvedValueOnce(settings())
      .mockResolvedValueOnce(settings({ patrol_model: 'anthropic:claude-haiku-4-5' }));
    mocks.getModels.mockResolvedValue({
      models: [
        { id: 'anthropic:claude-haiku-4-5-20251001', name: 'Haiku', provider: 'anthropic' },
        { id: 'anthropic:claude-haiku-4-5', name: 'Haiku', provider: 'anthropic' },
        { id: 'anthropic:claude-opus-4-1', name: 'Opus', provider: 'anthropic' },
      ],
    });
    mocks.runPatrolModelReadiness.mockResolvedValue({
      status: 'pass',
      max_verified_mode: 'approval',
      summary: '',
    } as PatrolModelReadinessSnapshot);
    const props = renderPanel();
    await screen.findByText('claude-haiku-4-5');

    const key = screen.getByLabelText('Anthropic API key') as HTMLInputElement;
    fireEvent.input(key, { target: { value: ' sk-ant-test ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Connect and check' }));

    expect(await screen.findByTestId('patrol-setup-ready')).toHaveTextContent('Patrol is ready');
    expect(props.onStart).toHaveBeenCalledTimes(1);
    expect(mocks.updateSettings).toHaveBeenNthCalledWith(1, {
      enabled: true,
      anthropic_api_key: 'sk-ant-test',
    });
    expect(mocks.updateSettings).toHaveBeenNthCalledWith(2, {
      patrol_model: 'anthropic:claude-haiku-4-5',
      model: 'anthropic:claude-haiku-4-5',
      patrol_interval_minutes: 720,
    });
    expect(mocks.runPatrolModelReadiness).toHaveBeenCalledWith(
      { model: 'anthropic:claude-haiku-4-5' },
      expect.any(AbortSignal),
    );
    expect(mocks.syncAIRuntimeSettings).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole('button', { name: 'Run Patrol now' }));
    expect(props.onRunPatrol).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: 'Done' }));
    expect(props.onDone).toHaveBeenCalledTimes(1);
  });

  it('keeps a failed check on screen with the reason and a retry', async () => {
    mocks.updateSettings
      .mockResolvedValueOnce(settings())
      .mockResolvedValueOnce(settings({ patrol_model: 'ollama:qwen3:8b' }));
    mocks.getModels.mockResolvedValue({
      models: [{ id: 'ollama:qwen3:8b', name: 'qwen3:8b', provider: 'ollama' }],
    });
    mocks.getPatrolCostPreview.mockResolvedValue({ ...haikuProjection, billed_per_token: false });
    mocks.runPatrolModelReadiness.mockResolvedValue({
      status: 'fail',
      summary: 'The model made no tool call.',
      recommendation: 'Choose another model.',
    } as PatrolModelReadinessSnapshot);
    renderPanel();

    fireEvent.click(await screen.findByRole('button', { name: /Ollama/ }));
    expect(screen.getByLabelText('Ollama server URL')).toHaveValue('http://localhost:11434');
    expect(await screen.findByText(/ollama pull qwen3:8b/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Connect and check' }));

    const error = await screen.findByTestId('patrol-setup-error');
    expect(error).toHaveTextContent('The model did not pass Patrol’s check');
    expect(error).toHaveTextContent('The model made no tool call. Choose another model.');
    expect(mocks.updateSettings).toHaveBeenNthCalledWith(2, {
      patrol_model: 'ollama:qwen3:8b',
      model: 'ollama:qwen3:8b',
    });

    fireEvent.click(screen.getByRole('button', { name: 'Check qwen3:8b again' }));
    await waitFor(() => expect(mocks.runPatrolModelReadiness).toHaveBeenCalledTimes(2));
  });

  it('says to pull the model when Ollama has nothing Pulse can pick', async () => {
    mocks.updateSettings.mockResolvedValueOnce(settings());
    // Ollama answers, but the provider test's default model is not pulled.
    mocks.testProvider.mockResolvedValue({
      success: false,
      message: 'Selected model unavailable',
      provider: 'ollama',
      cause: 'model_unavailable',
    });
    mocks.getModels.mockResolvedValue({ models: [] });
    renderPanel();

    fireEvent.click(await screen.findByRole('button', { name: /Ollama/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Connect and check' }));

    expect(await screen.findByTestId('patrol-setup-error')).toHaveTextContent(
      'Pulse reached Ollama, but qwen3:8b is not installed there.',
    );
    expect(mocks.updateSettings).toHaveBeenCalledTimes(1);
    expect(mocks.runPatrolModelReadiness).not.toHaveBeenCalled();
  });

  it('reports a rejected key from the provider test instead of guessing a model', async () => {
    mocks.updateSettings.mockResolvedValueOnce(settings());
    mocks.testProvider.mockResolvedValue({
      success: false,
      message: 'Provider authentication issue',
      provider: 'anthropic',
      cause: 'provider_auth',
      summary: 'The provider rejected the configured credentials or account access.',
      recommendation: 'Check the API key on the Provider & Models settings page.',
    });
    renderPanel();
    await screen.findByText('claude-haiku-4-5');

    fireEvent.input(screen.getByLabelText('Anthropic API key'), {
      target: { value: 'sk-ant-bad' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Connect and check' }));

    expect(await screen.findByTestId('patrol-setup-error')).toHaveTextContent(
      'Anthropic did not accept the connection: The provider rejected the configured credentials or account access. Check the API key and connect again.',
    );
    expect(mocks.testProvider).toHaveBeenCalledWith('anthropic');
    expect(mocks.getModels).not.toHaveBeenCalled();
    expect(mocks.runPatrolModelReadiness).not.toHaveBeenCalled();
    // The saved key is not left in the page.
    expect(screen.getByLabelText('Anthropic API key')).toHaveValue('');
  });

  it('reports a model list failure after the provider answered', async () => {
    mocks.updateSettings.mockResolvedValueOnce(settings());
    mocks.getModels.mockResolvedValue({ models: [], error: 'Failed to fetch model list' });
    renderPanel();
    await screen.findByText('claude-haiku-4-5');

    fireEvent.input(screen.getByLabelText('Anthropic API key'), { target: { value: 'sk-ant-ok' } });
    fireEvent.click(screen.getByRole('button', { name: 'Connect and check' }));

    expect(await screen.findByTestId('patrol-setup-error')).toHaveTextContent(
      'Anthropic did not return its models: Failed to fetch model list',
    );
    expect(mocks.runPatrolModelReadiness).not.toHaveBeenCalled();
  });
});
