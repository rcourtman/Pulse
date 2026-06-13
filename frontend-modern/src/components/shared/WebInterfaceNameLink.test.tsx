import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { WebInterfaceNameLink } from './WebInterfaceNameLink';

describe('WebInterfaceNameLink', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders a safe web-interface launch link through the resource name', () => {
    const onRowClick = vi.fn();

    render(() => (
      <div onClick={onRowClick}>
        <WebInterfaceNameLink name="node-a" url="https://node-a.example.local:8006" />
      </div>
    ));

    const link = screen.getByRole('link', { name: 'Open web interface for node-a' });

    expect(link).toHaveAttribute('href', 'https://node-a.example.local:8006');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link).toHaveAttribute('title', 'Open https://node-a.example.local:8006');

    fireEvent.click(link);

    expect(onRowClick).not.toHaveBeenCalled();
  });

  it('falls back to inert resource text when no URL is available', () => {
    render(() => (
      <WebInterfaceNameLink
        name="node-b"
        url=" "
        class="link-class"
        fallbackClass="fallback-class"
      />
    ));

    const fallback = screen.getByText('node-b');

    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(fallback.tagName).toBe('SPAN');
    expect(fallback).toHaveClass('fallback-class');
    expect(fallback).not.toHaveClass('link-class');
    expect(fallback).toHaveAttribute('title', 'node-b');
  });
});
