import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import Docs from '../Docs';

const missing = "That document is not part of this installation's shipped documentation set.";
const access = 'Access to this guide was refused.';
const temporary = 'This guide could not be loaded.';
const privateBody = '<h1>private-upstream-detail</h1><script>window.leaked = true</script>';
const faq = '# FAQ\n\n## Answer\n\nA shipped answer.';
const markdown = (body: string) =>
  new Response(body, { headers: { 'Content-Type': 'text/plain' } });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};

function mount() {
  render(() => (
    <Router>
      <Route path="/docs/*docPath" component={Docs} />
    </Router>
  ));
}

describe('Docs load failures and explicit same-guide retry', () => {
  const fetchMock = vi.fn<typeof fetch>();
  const scroll = vi.fn();

  beforeEach(() => {
    window.history.replaceState(null, '', '/docs/FAQ?reader=fixture#answer');
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
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

  it.each([
    [404, 'text/plain', missing],
    [410, 'text/plain', missing],
    [200, 'text/html; charset=utf-8', missing],
    [401, 'text/html', access],
    [403, 'text/html', access],
    [408, 'text/plain', temporary],
    [429, 'text/plain', temporary],
    [500, 'text/html', temporary],
    [503, 'text/html', temporary],
  ])(
    'classifies HTTP %s / %s without exposing its response',
    async (status, contentType, message) => {
      fetchMock.mockResolvedValueOnce(
        new Response(privateBody, {
          status,
          headers: { 'Content-Type': contentType },
        }),
      );
      mount();
      const alert = await screen.findByRole('alert');
      expect(alert.textContent).toContain(message);
      expect(alert.textContent?.includes(missing)).toBe(message === missing);
      expect(document.body).not.toHaveTextContent('private-upstream-detail');
      expect(document.querySelector('article')).toBeNull();
      expect(screen.getByRole('button', { name: 'Try again' })).toBeEnabled();
      expect(fetchMock).toHaveBeenCalledExactlyOnceWith('/docs/FAQ.md', {
        headers: { Accept: 'text/markdown, text/plain' },
      });
    },
  );

  it.each(['connection', 'body'])(
    'handles an interrupted %s as retryable, not missing',
    async (phase) => {
      if (phase === 'connection') fetchMock.mockRejectedValueOnce(new Error(privateBody));
      else
        fetchMock.mockResolvedValueOnce({
          ok: true,
          headers: new Headers({ 'Content-Type': 'text/markdown' }),
          text: () => Promise.reject(new Error(privateBody)),
        } as Response);
      mount();
      expect(await screen.findByRole('alert')).toHaveTextContent(temporary);
      expect(document.body).not.toHaveTextContent('private-upstream-detail');
      expect(document.body).not.toHaveTextContent(missing);
      expect(fetchMock).toHaveBeenCalledTimes(1);
    },
  );

  it('announces initial loading, then renders ordinary shipped markdown', async () => {
    const pending = deferred<Response>();
    fetchMock.mockReturnValueOnce(pending.promise);
    mount();
    expect(screen.getByRole('status')).toHaveTextContent('Loading documentation');
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
    pending.resolve(markdown(faq));
    // FAQ is also the loading title fallback; wait for the fetched section.
    const heading = await screen.findByRole('heading', { name: 'Answer' });
    expect(screen.getByRole('heading', { name: 'FAQ' })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).toBeNull();
    await waitFor(() => expect(document.activeElement).toBe(heading));
  });

  it('retries only on request, blocks duplicate activation and restores the exact guide/section', async () => {
    const pending = deferred<Response>();
    fetchMock.mockRejectedValueOnce(new Error(privateBody)).mockReturnValueOnce(pending.promise);
    mount();
    const button = await screen.findByRole('button', { name: 'Try again' });
    const location = window.location.href;
    const historyLength = window.history.length;
    button.focus();
    fireEvent.click(button);
    expect(button).toBeDisabled();
    expect(document.activeElement).toBe(button);
    expect(screen.getByRole('status')).toHaveTextContent('Loading documentation');
    button.click();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[1]).toEqual(fetchMock.mock.calls[0]);
    pending.resolve(markdown(faq));
    const heading = await screen.findByRole('heading', { name: 'Answer' });
    await waitFor(() => expect(document.activeElement).toBe(heading));
    expect(scroll).toHaveBeenCalledWith({ block: 'start' });
    expect(window.location.href).toBe(location);
    expect(window.history.length).toBe(historyLength);
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('keeps the focused retry control usable after another failed read', async () => {
    const pending = deferred<Response>();
    fetchMock
      .mockResolvedValueOnce(new Response('', { status: 403 }))
      .mockReturnValueOnce(pending.promise);
    mount();
    const button = await screen.findByRole('button', { name: 'Try again' });
    button.focus();
    fireEvent.click(button);
    pending.resolve(new Response(privateBody, { status: 503 }));
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(temporary));
    expect(button).toBeEnabled();
    expect(document.activeElement).toBe(button);
    expect(screen.queryByRole('status')).toBeNull();
    expect(document.body).not.toHaveTextContent('private-upstream-detail');
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('allows index navigation during retry and ignores the obsolete response', async () => {
    const oldRead = deferred<Response>();
    const indexRead = deferred<Response>();
    fetchMock
      .mockRejectedValueOnce(new Error(privateBody))
      .mockReturnValueOnce(oldRead.promise)
      .mockReturnValueOnce(indexRead.promise);
    mount();
    fireEvent.click(await screen.findByRole('button', { name: 'Try again' }));
    fireEvent.click(screen.getByRole('link', { name: 'Back to the documentation index' }));
    await waitFor(() => expect(window.location.pathname).toBe('/docs/README'));
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByRole('status')).toHaveTextContent('Loading documentation');
    indexRead.resolve(markdown('# Documentation\n\nExisting index.'));
    expect(await screen.findByRole('heading', { name: 'Documentation' })).toBeInTheDocument();
    oldRead.resolve(markdown(faq));
    await oldRead.promise;
    expect(screen.queryByRole('heading', { name: 'FAQ' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
    expect(document.body).toHaveTextContent('Existing index.');
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      '/docs/FAQ.md',
      '/docs/FAQ.md',
      '/docs/README.md',
    ]);
  });
});
