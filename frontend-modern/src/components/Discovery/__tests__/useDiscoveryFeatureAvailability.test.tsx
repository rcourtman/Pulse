import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const aiApiMocks = vi.hoisted(() => ({
  getSettings: vi.fn(),
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getSettings: aiApiMocks.getSettings,
  },
}));

import { resetAIRuntimeState } from '@/stores/aiRuntimeState';
import { useDiscoveryFeatureAvailability } from '../useDiscoveryFeatureAvailability';

const AvailabilityProbe = () => {
  const discovery = useDiscoveryFeatureAvailability();
  return (
    <div>
      <span data-testid="resolved">{String(discovery.discoveryFeatureResolved())}</span>
      <span data-testid="enabled">{String(discovery.discoveryFeatureEnabled())}</span>
      <span data-testid="disabled">{String(discovery.discoveryFeatureKnownDisabled())}</span>
    </div>
  );
};

beforeEach(() => {
  resetAIRuntimeState();
  aiApiMocks.getSettings.mockReset();
});

afterEach(() => {
  cleanup();
  resetAIRuntimeState();
});

describe('useDiscoveryFeatureAvailability', () => {
  it('stays hidden until Discovery is explicitly enabled', async () => {
    aiApiMocks.getSettings.mockResolvedValue({ discovery_enabled: true });

    render(() => <AvailabilityProbe />);

    expect(screen.getByTestId('enabled')).toHaveTextContent('false');
    await waitFor(() => expect(screen.getByTestId('resolved')).toHaveTextContent('true'));
    expect(screen.getByTestId('enabled')).toHaveTextContent('true');
    expect(screen.getByTestId('disabled')).toHaveTextContent('false');
  });

  it('classifies disabled and missing settings as unavailable', async () => {
    aiApiMocks.getSettings.mockResolvedValue({ discovery_enabled: false });

    render(() => <AvailabilityProbe />);

    await waitFor(() => expect(screen.getByTestId('resolved')).toHaveTextContent('true'));
    expect(screen.getByTestId('enabled')).toHaveTextContent('false');
    expect(screen.getByTestId('disabled')).toHaveTextContent('true');
  });

  it('fails closed when AI settings cannot be loaded', async () => {
    aiApiMocks.getSettings.mockRejectedValue(new Error('settings unavailable'));

    render(() => <AvailabilityProbe />);

    await waitFor(() => expect(screen.getByTestId('resolved')).toHaveTextContent('true'));
    expect(screen.getByTestId('enabled')).toHaveTextContent('false');
    expect(screen.getByTestId('disabled')).toHaveTextContent('true');
  });
});
