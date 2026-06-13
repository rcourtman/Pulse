import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import discoveryLoadingFallbackSource from '@/components/shared/DiscoveryLoadingFallback.tsx?raw';
import { DiscoveryLoadingFallback } from './DiscoveryLoadingFallback';

describe('DiscoveryLoadingFallback', () => {
  it('renders the shared discovery loading row on the canonical LoadingSpinner primitive', () => {
    render(() => <DiscoveryLoadingFallback text="Loading discovery details" />);

    const status = screen.getByRole('status');
    expect(status).toHaveTextContent('Loading discovery details');
    expect(status).toHaveClass('flex');
    expect(status).toHaveClass('items-center');
    expect(status).toHaveClass('justify-center');
    expect(status).toHaveClass('py-8');
    expect(status.querySelector('span[aria-hidden="true"]')).toHaveClass('h-6');
    expect(status.querySelector('span[aria-hidden="true"]')).toHaveClass('border-blue-500');

    expect(discoveryLoadingFallbackSource).toContain('LoadingSpinner');
    expect(discoveryLoadingFallbackSource).not.toContain('animate-spin h-6 w-6');
  });
});
