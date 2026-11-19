import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';

const [guideAvailable, setGuideAvailable] = createSignal(false);

vi.mock('@/utils/featureReleases', () => ({
  isFeatureGuideAvailable: () => guideAvailable(),
}));

import Docs from '../Docs';

const missing = "That document is not part of this installation's shipped documentation set.";
const guide = '# Map\n\n## Tiles\n\nWhat each tile is.';

function mount() {
  render(() => (
    <Router>
      <Route path="/docs/*docPath" component={Docs} />
    </Router>
  ));
}

describe('the guide of a feature this build does not show', () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    setGuideAvailable(false);
    window.history.replaceState(null, '', '/docs/MAP');
    fetchMock.mockReset();
    fetchMock.mockImplementation(async () => {
      return new Response(guide, { headers: { 'Content-Type': 'text/plain' } });
    });
    vi.stubGlobal('fetch', fetchMock);
    vi.stubGlobal('scrollTo', vi.fn());
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    window.history.replaceState(null, '', '/');
  });

  it('is not part of the shipped set and is never fetched', async () => {
    mount();

    expect(await screen.findByText(missing)).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(screen.queryByText('What each tile is.')).not.toBeInTheDocument();
  });

  it('opens once the build shows the feature', async () => {
    mount();
    expect(await screen.findByText(missing)).toBeInTheDocument();

    setGuideAvailable(true);

    expect(await screen.findByText('What each tile is.')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText(missing)).not.toBeInTheDocument());
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
