import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import Docs from '../Docs';

const markdown = `# Fixture help

## Cost Tracking

[Bare return](#cost-tracking)
[File **return**](AI.md#cost-tracking)
[Other section](#other-section)
[Other document](FAQ.md#answer)
[External](https://example.invalid/help)

## Other section

Reader position.
`;

describe('Docs fragment activation through the router', () => {
  const scroll = vi.fn();

  beforeEach(() => {
    window.history.replaceState(null, '', '/docs/AI#cost-tracking');
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async (url: string) =>
          new Response(
            url.includes('FAQ.md') ? '# FAQ\n\n## Answer\n\nExisting answer.' : markdown,
            { headers: { 'Content-Type': 'text/markdown' } },
          ),
      ),
    );
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) =>
      setTimeout(() => callback(0), 0),
    );
    vi.stubGlobal('cancelAnimationFrame', (id: number) => clearTimeout(id));
    vi.stubGlobal('scrollTo', vi.fn());
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scroll,
    });
    scroll.mockClear();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView');
    window.history.replaceState(null, '', '/');
  });

  async function mount() {
    render(() => (
      <Router>
        <Route path="/docs/*docPath" component={Docs} />
      </Router>
    ));
    const heading = await screen.findByRole('heading', { name: 'Cost Tracking' });
    await waitFor(() => expect(document.activeElement).toBe(heading));
    scroll.mockClear();
    return heading;
  }

  it.each(['Bare return', 'File return'])(
    'restores scroll and focus on a repeated %s',
    async (label) => {
      const heading = await mount();
      const link = screen.getByRole('link', { name: label });
      link.focus();
      const historyLength = window.history.length;

      fireEvent.click(link.querySelector('strong') ?? link);

      await waitFor(() => expect(document.activeElement).toBe(heading));
      expect(scroll).toHaveBeenCalledWith({ block: 'start' });
      expect(window.location.hash).toBe('#cost-tracking');
      expect(window.history.length).toBe(historyLength);
      expect(fetch).toHaveBeenCalledTimes(1);
    },
  );

  it('routes changed hashes and other documents, then focuses their rendered headings', async () => {
    await mount();
    fireEvent.click(screen.getByRole('link', { name: 'Other section' }));
    const other = screen.getByRole('heading', { name: 'Other section' });
    await waitFor(() => expect(document.activeElement).toBe(other));
    expect(window.location.hash).toBe('#other-section');
    expect(fetch).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('link', { name: 'Other document' }));
    const answer = await screen.findByRole('heading', { name: 'Answer' });
    await waitFor(() => expect(document.activeElement).toBe(answer));
    expect(window.location.pathname).toBe('/docs/FAQ');
    expect(window.location.hash).toBe('#answer');
    expect(fetch).toHaveBeenCalledTimes(2);
  });

  it('keeps a bare fragment relative to the current query as well as document', async () => {
    window.history.replaceState(null, '', '/docs/AI?view=fixture#cost-tracking');
    await mount();
    fireEvent.click(screen.getByRole('link', { name: 'Other section' }));
    await waitFor(() => expect(window.location.hash).toBe('#other-section'));
    expect(window.location.search).toBe('?view=fixture');
  });

  it.each(['ctrlKey', 'metaKey', 'shiftKey', 'altKey'])(
    'leaves %s activation to the browser',
    async (key) => {
      await mount();
      const link = screen.getByRole('link', { name: 'File return' });
      link.focus();
      const event = new MouseEvent('click', { bubbles: true, cancelable: true, [key]: true });
      // Stop only the native default in this synthetic DOM, after both handlers.
      const inspect = vi.fn((observed: MouseEvent) => {
        expect(observed.defaultPrevented).toBe(false);
        observed.preventDefault();
      });
      document.addEventListener('click', inspect, { once: true });
      link.dispatchEvent(event);
      expect(inspect).toHaveBeenCalledOnce();
      expect(document.activeElement).toBe(link);
      expect(scroll).not.toHaveBeenCalled();
    },
  );

  it('leaves external links isolated rather than routing them into the viewer', async () => {
    await mount();
    const link = screen.getByRole('link', { name: 'External' });
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(link).not.toHaveAttribute('data-doc-link');
  });
});
