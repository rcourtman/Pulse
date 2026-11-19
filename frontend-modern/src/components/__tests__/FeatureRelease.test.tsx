import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const [availability, setAvailability] = createSignal<'on' | 'off' | 'pending'>('off');
const [inPreview, setInPreview] = createSignal(false);

vi.mock('@/utils/featureReleases', () => ({
  getFeatureAvailability: () => availability(),
  isFeatureInPreview: () => inPreview(),
}));

import {
  FEATURE_PENDING_LIMIT_MS,
  FeaturePreviewTag,
  PreviewTag,
  featureReleaseRoute,
} from '../FeatureRelease';

const pageLoaded = vi.fn();
const Page = () => {
  pageLoaded();
  return <div>The feature</div>;
};
const Unavailable = () => <div>Page not found</div>;

describe('FeatureRelease', () => {
  beforeEach(() => {
    setAvailability('off');
    setInPreview(false);
    pageLoaded.mockClear();
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('answers like an unknown address where the feature is off, without loading it', () => {
    const Gated = featureReleaseRoute('infrastructureMap', Page, Unavailable);
    render(() => <Gated />);

    expect(screen.getByText('Page not found')).toBeInTheDocument();
    expect(screen.queryByText('The feature')).not.toBeInTheDocument();
    expect(pageLoaded).not.toHaveBeenCalled();
  });

  it('draws the feature where it is on', () => {
    setAvailability('on');
    const Gated = featureReleaseRoute('infrastructureMap', Page, Unavailable);
    render(() => <Gated />);

    expect(screen.getByText('The feature')).toBeInTheDocument();
    expect(screen.queryByText('Page not found')).not.toBeInTheDocument();
  });

  it('draws nothing while the build is unknown, then the feature once it is a Preview build', () => {
    setAvailability('pending');
    const Gated = featureReleaseRoute('infrastructureMap', Page, Unavailable);
    render(() => <Gated />);

    expect(screen.queryByText('The feature')).not.toBeInTheDocument();
    expect(screen.queryByText('Page not found')).not.toBeInTheDocument();

    setAvailability('on');
    expect(screen.getByText('The feature')).toBeInTheDocument();
  });

  it('answers as unknown if the build never says what it is, and still turns on if it does later', () => {
    vi.useFakeTimers();
    setAvailability('pending');
    const Gated = featureReleaseRoute('infrastructureMap', Page, Unavailable);
    render(() => <Gated />);

    vi.advanceTimersByTime(FEATURE_PENDING_LIMIT_MS - 1);
    expect(screen.queryByText('Page not found')).not.toBeInTheDocument();

    vi.advanceTimersByTime(1);
    expect(screen.getByText('Page not found')).toBeInTheDocument();
    expect(pageLoaded).not.toHaveBeenCalled();

    setAvailability('on');
    expect(screen.getByText('The feature')).toBeInTheDocument();
    expect(screen.queryByText('Page not found')).not.toBeInTheDocument();
  });

  it('tags a feature only while it is in preview', () => {
    render(() => <FeaturePreviewTag feature="infrastructureMap" />);
    expect(screen.queryByText('Preview')).not.toBeInTheDocument();

    setInPreview(true);
    expect(screen.getByText('Preview')).toHaveAttribute(
      'title',
      'New in Preview builds. It can change before it is released.',
    );
  });

  it('draws the Preview mark the header uses', () => {
    render(() => <PreviewTag />);
    expect(screen.getByText('Preview')).toHaveClass('bg-orange-700', 'text-white');
  });
});
