import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { ExternalTextLink, getExternalTextLinkRel } from './ExternalTextLink';

describe('ExternalTextLink', () => {
  afterEach(() => {
    cleanup();
  });

  it('opens external text links with canonical new-tab safety', () => {
    render(() => <ExternalTextLink href="https://docs.example.test">Read docs</ExternalTextLink>);

    const link = screen.getByRole('link', { name: 'Read docs' });
    expect(link).toHaveAttribute('href', 'https://docs.example.test');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link.className).toContain('text-blue-600');
  });

  it('supports compact settings action links without local anchor shells', () => {
    render(() => (
      <ExternalTextLink href="https://docs.example.test/key" variant="compact">
        Get your API key
      </ExternalTextLink>
    ));

    const link = screen.getByRole('link', { name: 'Get your API key' });
    expect(link.className).toContain('min-h-10');
    expect(link.className).toContain('sm:min-h-9');
  });

  it('centralizes the preserve-opener exception', () => {
    expect(getExternalTextLinkRel()).toBe('noopener noreferrer');
    expect(getExternalTextLinkRel(true)).toBeUndefined();
  });
});
