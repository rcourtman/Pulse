/**
 * Which builds show a new feature.
 *
 * Work lands on `main` at once and `main` ships every week, so a new feature
 * (a new page, or a new idea users must learn) is listed in
 * `featureReleases.json` from its first pull request and moves through three
 * stages there:
 *
 *   build     development builds only: the development bundle. Nobody
 *             outside the project sees it.
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
 * `getFeatureGuideAvailability` before it opens a guide. Once a released
 * feature has shipped in a stable release, its entry and its checks are
 * removed.
 *
 * The end-to-end suite drives a production bundle, where a feature that is
 * not released does not exist. A spec for such a feature marks its own page
 * as a development build by adding `featureReleaseTestBuild: true` to that
 * page's `/api/version` reply. No Pulse server sends that field, so no
 * server can show a held feature, and every other spec still sees what
 * ships. The server's own `isDevelopment` is not used for this: it is also
 * true for an install made from unreleased source with the installer, and
 * those are users.
 */
import { updateStore } from '@/stores/updates';
import registry from './featureReleases.json';

export type FeatureId = keyof typeof registry;
export type FeatureStage = 'build' | 'preview' | 'released';
/** `pending` is a feature not in every build, before the build has said what it is. */
export type FeatureAvailability = 'on' | 'off' | 'pending';

export type FeatureBuild = {
  /**
   * A development build: the development bundle, or a page the end-to-end
   * suite has marked as one. Undefined until the version reply has arrived.
   */
  development: boolean | undefined;
  /** A published beta or release candidate. Undefined until the server has said. */
  preview: boolean | undefined;
};

// The server's wire value for a published pre-release build of any kind.
const PREVIEW_BUILD_CHANNEL = 'rc';
// The mark a spec adds to its own page's version reply. No server sends it.
export const FEATURE_RELEASE_TEST_BUILD_FIELD = 'featureReleaseTestBuild';

/** Anything the list does not spell exactly is treated as not yet shown. */
export const normalizeFeatureStage = (value: unknown): FeatureStage =>
  value === 'preview' || value === 'released' ? value : 'build';

export const getFeatureStage = (feature: FeatureId): FeatureStage =>
  normalizeFeatureStage(registry[feature].stage);

export const resolveFeatureAvailability = (
  stage: FeatureStage,
  build: FeatureBuild,
): FeatureAvailability => {
  if (stage === 'released' || build.development === true) return 'on';
  if (stage === 'preview' && build.preview === true) return 'on';
  // Not known yet whether this is a build that shows it.
  if (build.development === undefined) return 'pending';
  if (stage === 'preview' && build.preview === undefined) return 'pending';
  return 'off';
};

// What the version reply says this build is. A stable build says `stable`
// and a source build has no channel, so both are off for a feature in
// preview, and no server's reply makes a development build. If the version
// cannot be read, the feature stays off.
const readServerBuild = (): Pick<FeatureBuild, 'development' | 'preview'> => {
  const version = updateStore.versionInfo();
  if (version) {
    const reply = version as unknown as Record<string, unknown>;
    return {
      development: reply[FEATURE_RELEASE_TEST_BUILD_FIELD] === true,
      preview: version.channel === PREVIEW_BUILD_CHANNEL,
    };
  }
  return updateStore.lastError()
    ? { development: false, preview: false }
    : { development: undefined, preview: undefined };
};

/** Reactive: a feature not in every build turns on when the build's version says so. */
export const getFeatureAvailability = (feature: FeatureId): FeatureAvailability => {
  const stage = getFeatureStage(feature);
  // The server is asked only when the answer depends on it.
  if (stage === 'released' || import.meta.env.DEV) return 'on';
  return resolveFeatureAvailability(stage, readServerBuild());
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

/** `off` for the guide of a feature this build does not show, `pending` until that is known. */
export const getFeatureGuideAvailability = (docPath: string): FeatureAvailability => {
  const feature = getGuideFeature(docPath);
  return feature === undefined ? 'on' : getFeatureAvailability(feature);
};

/** False for the guide of a feature this build does not show. */
export const isFeatureGuideAvailable = (docPath: string): boolean =>
  getFeatureGuideAvailability(docPath) === 'on';

/** True while the feature's page carries the Preview tag. */
export const isFeatureInPreview = (feature: FeatureId): boolean =>
  getFeatureStage(feature) === 'preview';
