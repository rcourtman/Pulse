import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ConnectionsExplainer } from '../ConnectionsExplainer';

const DISMISS_KEY = 'pulse.infrastructure.explainer.dismissed.v1';

describe('ConnectionsExplainer', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    window.localStorage.clear();
  });

  it('names both ingestion modes with their branded labels', () => {
    render(() => <ConnectionsExplainer />);

    expect(screen.getByText('Platform API')).toBeInTheDocument();
    expect(screen.getByText('Pulse Unified Agent')).toBeInTheDocument();
  });

  it('hides itself and persists dismissal to localStorage when closed', () => {
    const { container } = render(() => <ConnectionsExplainer />);

    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));

    expect(container.querySelector('section')).toBeNull();
    expect(window.localStorage.getItem(DISMISS_KEY)).toBe('1');
  });

  it('stays dismissed across remounts once localStorage records it', () => {
    window.localStorage.setItem(DISMISS_KEY, '1');
    const { container } = render(() => <ConnectionsExplainer />);

    expect(container.querySelector('section')).toBeNull();
  });
});
