import { describe, expect, it } from 'vitest';
import {
  isInfrastructureMapAvailable,
  resolveInfrastructureMapAvailable,
} from '../infrastructureMapRelease';

describe('infrastructure map release gate', () => {
  it('keeps the map out of a release build until it is released', () => {
    expect(resolveInfrastructureMapAvailable(false, false)).toBe(false);
    expect(resolveInfrastructureMapAvailable(false, true)).toBe(true);
  });

  it('shows the map in a development build, where its review is done', () => {
    expect(resolveInfrastructureMapAvailable(true, false)).toBe(true);
    // The test runner is a development build.
    expect(isInfrastructureMapAvailable()).toBe(true);
  });
});
