import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import RuntimeHome from '@/pages/RuntimeHome';

describe('RuntimeHome', () => {
  it('defers workspace routing to the authenticated app shell', () => {
    render(() => <RuntimeHome />);

    expect(screen.getByText('Opening workspace...')).toBeInTheDocument();
  });
});
