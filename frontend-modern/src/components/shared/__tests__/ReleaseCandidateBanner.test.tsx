import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { ReleaseCandidateBanner } from '@/components/shared/ReleaseCandidateBanner';

afterEach(() => {
  cleanup();
});

describe('ReleaseCandidateBanner', () => {
  it('renders RC guidance with release-note and feedback links', () => {
    render(() => <ReleaseCandidateBanner version="6.0.0-rc.1" />);

    expect(screen.getByText('Pulse 6.0.0-rc.1 is the first public v6 RC.')).toBeInTheDocument();
    expect(
      screen.getByText(
        /Start in a staging or non-critical environment first, then send feedback/i,
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'View release notes' })).toHaveAttribute(
      'href',
      'https://github.com/rcourtman/Pulse/releases/tag/v6.0.0-rc.1',
    );
    expect(screen.getByRole('link', { name: 'Send feedback' })).toHaveAttribute(
      'href',
      'https://github.com/rcourtman/Pulse/issues/new?template=v6_rc_feedback.yml',
    );
  });
});
