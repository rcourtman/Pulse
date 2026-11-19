import { Match, Show, Switch, createSignal, onCleanup, type Component } from 'solid-js';
import {
  getFeatureAvailability,
  isFeatureInPreview,
  type FeatureId,
} from '@/utils/featureReleases';

const PREVIEW_TAG_CLASS = 'text-xs px-1.5 py-0.5 bg-orange-700 text-white rounded-sm font-bold';

/** The one Preview mark: beside the brand on a Preview build, beside a feature in preview. */
export function PreviewTag(props: { title?: string }) {
  return (
    <span class={PREVIEW_TAG_CLASS} title={props.title}>
      Preview
    </span>
  );
}

/** Beside a feature's page title. Draws nothing unless the feature is in preview. */
export function FeaturePreviewTag(props: { feature: FeatureId }) {
  return (
    <Show when={isFeatureInPreview(props.feature)}>
      <PreviewTag title="New in Preview builds. It can change before it is released." />
    </Show>
  );
}

/**
 * A route for a feature that is not yet in every build. Where the feature is
 * off the address behaves like any unknown one, and the feature's own code is
 * never loaded. Nothing is drawn while a preview feature waits for the build
 * to say what it is; if the build has not said after ten seconds the address
 * answers as unknown, and the feature still appears if the answer comes later.
 */
export const FEATURE_PENDING_LIMIT_MS = 10_000;

export const featureReleaseRoute =
  (feature: FeatureId, Page: Component, Unavailable: Component): Component =>
  () => {
    const [waitedTooLong, setWaitedTooLong] = createSignal(false);
    const timer = setTimeout(() => setWaitedTooLong(true), FEATURE_PENDING_LIMIT_MS);
    onCleanup(() => clearTimeout(timer));
    const availability = () => {
      const current = getFeatureAvailability(feature);
      return current === 'pending' && waitedTooLong() ? 'off' : current;
    };
    return (
      <Switch>
        <Match when={availability() === 'on'}>
          <Page />
        </Match>
        <Match when={availability() === 'off'}>
          <Unavailable />
        </Match>
      </Switch>
    );
  };
