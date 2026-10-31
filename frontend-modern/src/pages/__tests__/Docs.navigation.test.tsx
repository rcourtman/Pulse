import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import Docs from '../Docs';

// Drive the route's reactive inputs directly here; consuming browser proof
// exercises the actual router and its asynchronous navigation transitions.
const [route, setRoute] = createSignal(new URL('http://localhost/docs/guide#existing-section'));
const navigateMock = vi.fn((href: string) => setRoute(new URL(href, 'http://localhost')));
vi.mock('@solidjs/router', () => ({
  useParams: () => ({
    get docPath() {
      return route().pathname.replace(/^\/docs\//, '');
    },
  }),
  useLocation: () => ({
    get pathname() {
      return route().pathname;
    },
    get search() {
      return route().search;
    },
    get hash() {
      return route().hash;
    },
  }),
  useNavigate: () => navigateMock,
}));

const source = `# Example guide

[Return to section](guide.md#existing-section)

[Other section](guide.md#other-section)

[Next guide](next.md#next-section)

## Existing section

Some text.

## Other section

More text.`;
const fetchMock = vi.fn();
const scrollIntoView = vi.fn();
const originalScroll = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'scrollIntoView');
const frames = new Map<number, FrameRequestCallback>();
let nextFrame = 0;
const flushFrames = () => {
  const pending = [...frames.values()];
  frames.clear();
  pending.forEach((callback) => callback(0));
};
const renderGuide = (fragment = 'existing-section') => {
  setRoute(new URL(`http://localhost/docs/guide#${fragment}`));
  return render(() => <Docs />);
};

beforeEach(() => {
  frames.clear();
  scrollIntoView.mockClear();
  fetchMock.mockReset();
  navigateMock.mockClear();
  fetchMock.mockResolvedValue(new Response(source, { headers: { 'content-type': 'text/plain' } }));
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
    frames.set(++nextFrame, callback);
    return nextFrame;
  });
  vi.stubGlobal('cancelAnimationFrame', (id: number) => frames.delete(id));
  vi.stubGlobal('scrollTo', vi.fn());
  Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
    configurable: true,
    value: scrollIntoView,
  });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  if (originalScroll)
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', originalScroll);
  else Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView');
});

describe('Docs section navigation', () => {
  it('returns focus and scroll to the current fragment on every explicit activation without refetching', async () => {
    renderGuide();
    const heading = await screen.findByRole('heading', { name: 'Existing section' });
    flushFrames();
    expect(heading).toHaveFocus();
    const link = screen.getByRole('link', { name: 'Return to section' });
    scrollIntoView.mockClear();
    for (let activation = 0; activation < 2; activation += 1) {
      link.focus();
      fireEvent.click(link);
      expect(heading).toHaveFocus();
      expect(scrollIntoView.mock.contexts[activation]).toBe(heading);
    }
    expect(scrollIntoView).toHaveBeenCalledTimes(2);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(route().hash).toBe('#existing-section');
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it('waits for an asynchronously fetched destination before focusing its fragment', async () => {
    let resolve!: (response: Response) => void;
    fetchMock.mockReturnValueOnce(new Promise<Response>((done) => (resolve = done)));
    renderGuide();
    expect(screen.getByText('Loading documentation…')).toBeInTheDocument();
    flushFrames();
    expect(scrollIntoView).not.toHaveBeenCalled();
    resolve(new Response(source, { headers: { 'content-type': 'text/markdown' } }));
    const heading = await screen.findByRole('heading', { name: 'Existing section' });
    flushFrames();
    expect(heading).toHaveFocus();
    expect(scrollIntoView.mock.contexts).toEqual([heading]);
  });

  it.each(['ctrlKey', 'metaKey', 'shiftKey', 'altKey'])(
    'leaves modified %s activation to the browser',
    async (modifier) => {
      renderGuide();
      await screen.findByRole('heading', { name: 'Existing section' });
      flushFrames();
      const link = screen.getByRole('link', { name: 'Return to section' });
      link.focus();
      scrollIntoView.mockClear();
      fireEvent.click(link, { [modifier]: true });
      expect(scrollIntoView).not.toHaveBeenCalled();
      expect(link).toHaveFocus();
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(navigateMock).not.toHaveBeenCalled();
    },
  );

  it('keeps changed-fragment and cross-document navigation on the existing async path', async () => {
    renderGuide();
    await screen.findByRole('heading', { name: 'Existing section' });
    flushFrames();
    fireEvent.click(screen.getByRole('link', { name: 'Other section' }));
    await waitFor(() => {
      flushFrames();
      expect(screen.getByRole('heading', { name: 'Other section' })).toHaveFocus();
    });
    fetchMock.mockResolvedValueOnce(new Response('# Next guide\n\n## Next section'));
    fireEvent.click(screen.getByRole('link', { name: 'Next guide' }));
    const next = await screen.findByRole('heading', { name: 'Next section' });
    flushFrames();
    expect(next).toHaveFocus();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it.each(['html fallback', 'read failure'])(
    'does not target prior content after %s while the next guide is unavailable',
    async (failure) => {
      renderGuide();
      await screen.findByRole('heading', { name: 'Existing section' });
      flushFrames();
      scrollIntoView.mockClear();
      fetchMock.mockResolvedValueOnce(
        failure === 'html fallback'
          ? new Response('<html>SPA fallback</html>', { headers: { 'content-type': 'text/html' } })
          : new Response('unavailable', { status: 503 }),
      );
      fireEvent.click(screen.getByRole('link', { name: 'Next guide' }));
      if (failure === 'html fallback') {
        await screen.findByText(
          "That document is not part of this installation's shipped documentation set.",
        );
      } else {
        // A 503 does not establish absence (#3102). This navigation regression
        // checks stale-content focus, not the separate load-recovery copy.
        await screen.findByRole('link', { name: 'Back to the documentation index' });
      }
      flushFrames();
      expect(scrollIntoView).not.toHaveBeenCalled();
      expect(screen.queryByRole('heading', { name: 'Existing section' })).not.toBeInTheDocument();
    },
  );
});
