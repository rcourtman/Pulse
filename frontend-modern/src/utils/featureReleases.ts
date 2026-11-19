/**
 * Which builds show a new feature.
 *
 * Work lands on `main` at once and `main` ships every week, so a new feature
 * (a new page, or a new idea users must learn) is listed in
 * `featureReleases.json` from its first pull request and moves through three
 * stages there:
 *
 *   build     development builds only. Nobody outside the project sees it.
 *   preview   development builds and Preview builds (betas and release
 *             candidates). Its page carries the Preview tag.
 *   released  every build.
 *
 * Only Richard moves a feature to `preview` or `released`, after its review
 * as one whole feature. The change is the `stage` line plus an `approval`
 * line that says who said so and when; `npm run lint:feature-releases`
 * refuses a stage past `build` without one. The same check refuses a link to
 * a feature's guide from the shipped guides until the feature is released.
 *
 * Every place the feature can be reached or spoken of asks
 * `isFeatureAvailable`: its route, its way in, its browser tab title, and
 * any wording about it or way back to it on other pages. The docs page asks
 * `isFeatureGuideAvailable` before it opens a guide. Once a released feature has shipped in a stable release, its
 * entry and its checks are removed.
 */
import { updateStore } from '@/stores/updates';
import registry from './featureReleases.json';

export type FeatureId = keyof typeof registry;
export type FeatureStage = 'build' | 'preview' | 'released';
/** `pending` is a preview feature before the build has said what it is. */
export type FeatureAvailability = 'on' | 'off' | 'pending';

export type FeatureBuild = {
  /** A development build of the browser bundle. */
  development: boolean;
  /** A published beta or release candidate. Undefined until the server has said. */
  preview: boolean | undefined;
};

// The server's wire value for a published pre-release build of any kind.
const PREVIEW_BUILD_CHANNEL = 'rc';

/** Anything the list does not spell exactly is treated as not yet shown. */
export const normalizeFeatureStage = (value: unknown): FeatureStage =>
  value === 'preview' || value === 'released' ? value : 'build';

export const getFeatureStage = (feature: FeatureId): FeatureStage =>
  normalizeFeatureStage(registry[feature].stage);

export const resolveFeatureAvailability = (
  stage: FeatureStage,
  build: FeatureBuild,
): FeatureAvailability => {
  if (stage === 'released' || build.development) return 'on';
  if (stage !== 'preview') return 'off';
  if (build.preview === undefined) return 'pending';
  return build.preview ? 'on' : 'off';
};

// A source build has no channel and a stable build says `stable`, so both are
// off. If the version cannot be read, the feature stays off.
const readPreviewBuild = (): boolean | undefined => {
  const version = updateStore.versionInfo();
  if (version) return version.channel === PREVIEW_BUILD_CHANNEL;
  return updateStore.lastError() ? false : undefined;
};

/** Reactive: a preview feature turns on when the build's version arrives. */
export const getFeatureAvailability = (feature: FeatureId): FeatureAvailability => {
  const stage = getFeatureStage(feature);
  const development = import.meta.env.DEV;
  // The version is read only when the answer depends on it.
  const preview = stage === 'preview' && !development ? readPreviewBuild() : undefined;
  return resolveFeatureAvailability(stage, { development, preview });
};

/** True where the feature may be reached and spoken of. */
export const isFeatureAvailable = (feature: FeatureId): boolean =>
  getFeatureAvailability(feature) === 'on';

/** The feature a guide belongs to, for a path such as `MAP`, `docs/MAP.md` or `map`. */
export const getGuideFeature = (docPath: string): FeatureId | undefined => {
  const name = (docPath.split(/[/\\]/).pop() ?? '').replace(/\.md$/i, '').toLowerCase();
  if (!name) return undefined;
  return (Object.keys(registry) as FeatureId[]).find((feature) =>
    registry[feature].guides.some((guide) => guide.replace(/\.md$/i, '').toLowerCase() === name),
  );
};

/** False for the guide of a feature this build does not show. */
export const isFeatureGuideAvailable = (docPath: string): boolean => {
  const feature = getGuideFeature(docPath);
  return feature === undefined || isFeatureAvailable(feature);
};

/** True while the feature's page carries the Preview tag. */
export const isFeatureInPreview = (feature: FeatureId): boolean =>
  getFeatureStage(feature) === 'preview';
