import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen, fireEvent } from '@solidjs/testing-library';
import { TokenRevealDialog } from '@/components/TokenRevealDialog';
import { showTokenReveal, dismissTokenReveal } from '@/stores/tokenReveal';

const mockRecord = {
  id: 'test-token',
  name: 'Integration token',
  prefix: 'abc123',
  suffix: '7890',
  createdAt: new Date().toISOString(),
};

describe('TokenRevealDialog', () => {
  afterEach(() => {
    dismissTokenReveal();
    cleanup();
  });

  it('renders and dismisses the token dialog', async () => {
    render(() => <TokenRevealDialog />);
    expect(screen.queryByText(/API token ready/i)).toBeNull();

    showTokenReveal({
      token: 'abc123token7890',
      record: mockRecord,
      source: 'test',
    });

    expect(await screen.findByText(/API token ready/i)).toBeInTheDocument();
    expect(screen.getByText(/abc123token7890/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /close token dialog/i }));
    expect(screen.queryByText(/API token ready/i)).toBeNull();
  });

  it('supports dismissal via escape key', async () => {
    render(() => <TokenRevealDialog />);

    showTokenReveal({
      token: 'escape-test-token',
      record: mockRecord,
      source: 'test',
    });

    expect(await screen.findByText(/API token ready/i)).toBeInTheDocument();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByText(/API token ready/i)).toBeNull();
  });
});
