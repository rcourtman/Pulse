/**
 * Whether this build shows the map.
 *
 * The map is on `main` and held back from release builds. It was built in two
 * days and has not had its completeness review: every theme, real browsers
 * and a phone, real estates of different shapes, empty and error states,
 * accessibility, a large estate, an end-to-end spec, guides that match it and
 * time in a beta. Until that review is signed off, a release build has no way
 * into the map and says nothing about it.
 *
 * A development build shows it, so the review can be done on the real page.
 * Releasing it is one deliberate change: set this to `true` in a commit that
 * says the review passed, restore the guide links it removed, and let it
 * reach a beta before any candidate.
 */
export const INFRASTRUCTURE_MAP_RELEASED = false;

export const resolveInfrastructureMapAvailable = (
  developmentBuild: boolean,
  released: boolean = INFRASTRUCTURE_MAP_RELEASED,
): boolean => released || developmentBuild;

/** True where the map may be reached and spoken of: a development build, until it is released. */
export const isInfrastructureMapAvailable = (): boolean =>
  resolveInfrastructureMapAvailable(import.meta.env.DEV);
