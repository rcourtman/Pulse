import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ResourceNameWithWebInterfaceLink } from './WebInterfaceLink';

describe('ResourceNameWithWebInterfaceLink', () => {
  afterEach(() => {
    cleanup();
  });

  it('keeps the name inert and exposes a separate safe launch control', () => {
    const onRowClick = vi.fn();

    render(() => (
      <div onClick={onRowClick}>
        <ResourceNameWithWebInterfaceLink name="node-a" url="https://node-a.example.local:8006" />
      </div>
    ));

    const name = screen.getByText('node-a');
    const link = screen.getByRole('link', { name: 'Open web interface for node-a' });

    expect(name.tagName).toBe('SPAN');
    expect(name.closest('a')).toBeNull();
    expect(link).toHaveAttribute('href', 'https://node-a.example.local:8006');
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link).toHaveAttribute('title', 'Open https://node-a.example.local:8006');
    expect(link).toHaveClass('min-h-6', 'min-w-6', 'shrink-0');

    fireEvent.click(link);
    fireEvent.keyDown(link, { key: 'Enter' });

    expect(onRowClick).not.toHaveBeenCalled();
  });

  it('renders only inert resource text when no URL is available', () => {
    render(() => (
      <ResourceNameWithWebInterfaceLink
        name="node-b"
        url=" "
        class="wrapper-class"
        nameClass="name-class"
      />
    ));

    const name = screen.getByText('node-b');

    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(name).toHaveClass('name-class');
    expect(name.parentElement).toHaveClass('wrapper-class');
    expect(name).toHaveAttribute('title', 'node-b');
  });

  it.each([
    'javascript:alert(document.domain)',
    'data:text/html,<script>alert(1)</script>',
    'ftp://node-b.example.local',
    'not a URL',
  ])('refuses unsafe or malformed URL %s and communicates the invalid state', (url) => {
    render(() => <ResourceNameWithWebInterfaceLink name="node-b" url={url} />);

    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(
      screen.getByRole('img', {
        name: 'Web interface URL for node-b is invalid',
      }),
    ).toHaveAttribute('data-web-interface-url-state', 'invalid');
    expect(screen.getByText('node-b').closest('a')).toBeNull();
  });
});
