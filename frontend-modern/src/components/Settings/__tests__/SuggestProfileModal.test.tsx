import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { SuggestProfileModal } from '../SuggestProfileModal';

const suggestProfileMock = vi.fn();
const getConfigSchemaMock = vi.fn();
const validateConfigMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const loggerErrorMock = vi.fn();

vi.mock('@/api/agentProfiles', () => ({
  AgentProfilesAPI: {
    suggestProfile: (...args: unknown[]) => suggestProfileMock(...args),
    getConfigSchema: (...args: unknown[]) => getConfigSchemaMock(...args),
    validateConfig: (...args: unknown[]) => validateConfigMock(...args),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
  },
}));

const renderModal = (props?: Partial<Parameters<typeof SuggestProfileModal>[0]>) => {
  const onClose = vi.fn();
  const onSuggestionAccepted = vi.fn();
  render(() => (
    <SuggestProfileModal
      onClose={props?.onClose ?? onClose}
      onSuggestionAccepted={props?.onSuggestionAccepted ?? onSuggestionAccepted}
    />
  ));
  return { onClose, onSuggestionAccepted };
};

beforeEach(() => {
  suggestProfileMock.mockReset();
  getConfigSchemaMock.mockReset();
  validateConfigMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  loggerErrorMock.mockReset();

  getConfigSchemaMock.mockResolvedValue([
    {
      key: 'enable_docker',
      type: 'bool',
      description: 'Enable Docker container monitoring',
      defaultValue: true,
      required: false,
    },
    {
      key: 'interval',
      type: 'duration',
      description: 'Polling interval for metrics collection',
      defaultValue: '30s',
      required: false,
    },
    {
      key: 'log_level',
      type: 'enum',
      description: 'Agent log verbosity level',
      defaultValue: 'info',
      required: false,
    },
  ]);
});

afterEach(() => {
  cleanup();
});

describe('SuggestProfileModal', () => {
  it('renders preview details, defaults, and validation hints', async () => {
    suggestProfileMock.mockResolvedValue({
      name: 'Production Servers',
      description: 'Tighter logging for prod hosts',
      config: {
        enable_docker: false,
        interval: '1m',
        mystery_key: 'value',
      },
      rationale: ['Reduce noise', 'Keep Docker off'],
    });

    validateConfigMock.mockResolvedValue({
      valid: false,
      errors: [{ key: 'interval', message: 'Expected duration string' }],
      warnings: [{ key: 'mystery_key', message: 'Unknown configuration key' }],
    });

    renderModal();

    fireEvent.input(
      screen.getByPlaceholderText('Describe the agents and use case for this profile...'),
      {
        target: { value: 'Production profile with minimal logging' },
      },
    );
    fireEvent.click(screen.getByRole('button', { name: /get ideas/i }));

    expect(await screen.findByText('Production Servers')).toBeInTheDocument();
    expect(screen.getByText('Settings Preview')).toBeInTheDocument();
    expect(screen.getByText('Enable Docker Monitoring')).toBeInTheDocument();
    expect(screen.getByText('Reporting Interval')).toBeInTheDocument();
    expect(screen.getByText('Unknown (ignored)')).toBeInTheDocument();

    expect(screen.getByText(/Fix these before saving/i)).toBeInTheDocument();
    expect(screen.getByText(/Review before saving/i)).toBeInTheDocument();
    expect(screen.getByText(/Review checklist/i)).toBeInTheDocument();
    expect(screen.getByText(/Docker monitoring is disabled/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /show defaults/i }));
    expect(await screen.findByText('log_level')).toBeInTheDocument();
  });

  it('supports regenerating and accepting a later draft', async () => {
    const firstSuggestion = {
      name: 'Production Profile',
      description: 'For prod workloads',
      config: { enable_docker: true },
      rationale: [],
    };
    const secondSuggestion = {
      name: 'Development Profile',
      description: 'For dev workloads',
      config: { log_level: 'debug' },
      rationale: [],
    };

    suggestProfileMock
      .mockResolvedValueOnce(firstSuggestion)
      .mockResolvedValueOnce(secondSuggestion);
    validateConfigMock.mockResolvedValue({ valid: true, errors: [], warnings: [] });

    const { onSuggestionAccepted } = renderModal();

    const promptInput = screen.getByPlaceholderText('Describe the agents and use case for this profile...');
    fireEvent.input(promptInput, { target: { value: 'Production profile' } });
    fireEvent.click(screen.getByRole('button', { name: /get ideas/i }));

    expect(await screen.findByText('Production Profile')).toBeInTheDocument();

    fireEvent.input(promptInput, { target: { value: 'Development profile' } });
    fireEvent.click(screen.getByRole('button', { name: /try again/i }));

    expect(await screen.findByText('Development Profile')).toBeInTheDocument();
    expect(screen.getByText('Recent drafts')).toBeInTheDocument();

    expect(screen.getByText('Production Profile')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /use this profile/i }));

    await waitFor(() => {
      expect(onSuggestionAccepted).toHaveBeenCalledWith(secondSuggestion);
    });
  });
});
