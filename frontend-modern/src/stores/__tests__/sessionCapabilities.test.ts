import { afterEach, describe, expect, it } from 'vitest';
import {
  sessionCapabilities,
  sessionCapabilitiesResolved,
  syncSessionCapabilities,
} from '@/stores/sessionCapabilities';

const resetSessionCapabilities = () => {
  syncSessionCapabilities(null);
};

describe('session capabilities store', () => {
  afterEach(() => {
    resetSessionCapabilities();
  });

  it('default-fills capabilities when no status payload is provided', () => {
    const next = syncSessionCapabilities();

    expect(next).toEqual({ demoMode: false, businessEstate: false });
    expect(sessionCapabilities()).toEqual({ demoMode: false, businessEstate: false });
  });

  it('default-fills capabilities when the status payload is null', () => {
    const next = syncSessionCapabilities(null);

    expect(next).toEqual({ demoMode: false, businessEstate: false });
    expect(sessionCapabilities()).toEqual({ demoMode: false, businessEstate: false });
  });

  it('default-fills capabilities when sessionCapabilities is omitted from status', () => {
    const next = syncSessionCapabilities({});

    expect(next).toEqual({ demoMode: false, businessEstate: false });
    expect(sessionCapabilities()).toEqual({ demoMode: false, businessEstate: false });
  });

  it('default-fills capabilities when sessionCapabilities is explicitly undefined', () => {
    const next = syncSessionCapabilities({ sessionCapabilities: undefined });

    expect(next).toEqual({ demoMode: false, businessEstate: false });
  });

  it('default-fills capabilities when sessionCapabilities is null', () => {
    const next = syncSessionCapabilities({ sessionCapabilities: null } as unknown as Parameters<
      typeof syncSessionCapabilities
    >[0]);

    expect(next).toEqual({ demoMode: false, businessEstate: false });
  });

  it('preserves an explicit demoMode===true capability', () => {
    const next = syncSessionCapabilities({
      sessionCapabilities: { demoMode: true },
    });

    expect(next).toEqual({ demoMode: true, businessEstate: false });
    expect(sessionCapabilities()).toEqual({ demoMode: true, businessEstate: false });
  });

  it('coerces an explicit demoMode===false capability to the default', () => {
    const next = syncSessionCapabilities({
      sessionCapabilities: { demoMode: false },
    });

    expect(next).toEqual({ demoMode: false, businessEstate: false });
  });

  // The normalizer gates demoMode on strict equality with `true`, so any
  // truthy value that is not the boolean `true` must collapse to false.
  it.each([
    ['number 1', 1],
    ['string "true"', 'true'],
    ['string "yes"', 'yes'],
    ['empty object', {}],
    ['array', [1]],
  ])('coerces a truthy-but-not-true demoMode (%s) to false', (_label, demoMode) => {
    const next = syncSessionCapabilities({
      sessionCapabilities: { demoMode: demoMode as unknown as boolean },
    });

    expect(next).toEqual({ demoMode: false, businessEstate: false });
  });

  it.each([
    ['zero', 0],
    ['empty string', ''],
    ['null', null],
    ['undefined', undefined],
    ['NaN', Number.NaN],
  ])('coerces a falsy demoMode (%s) to false', (_label, demoMode) => {
    const next = syncSessionCapabilities({
      sessionCapabilities: { demoMode: demoMode as unknown as boolean },
    });

    expect(next).toEqual({ demoMode: false, businessEstate: false });
  });

  it('returns the same normalized value it publishes to the signal', () => {
    const next = syncSessionCapabilities({
      sessionCapabilities: { demoMode: true },
    });

    expect(sessionCapabilities()).toBe(next);
  });

  it('marks session capabilities as resolved after a sync', () => {
    syncSessionCapabilities(null);

    expect(sessionCapabilitiesResolved()).toBe(true);
  });

  it('overwrites the previous capability value instead of merging', () => {
    syncSessionCapabilities({ sessionCapabilities: { demoMode: true } });
    expect(sessionCapabilities()).toEqual({ demoMode: true, businessEstate: false });

    const next = syncSessionCapabilities(null);

    expect(next).toEqual({ demoMode: false, businessEstate: false });
    expect(sessionCapabilities()).toEqual({ demoMode: false, businessEstate: false });
  });

  it('strips unrecognized capability fields rather than passing them through', () => {
    const next = syncSessionCapabilities({
      sessionCapabilities: {
        demoMode: true,
        assistantEnabled: true,
      },
    });

    expect(next).toEqual({ demoMode: true, businessEstate: false });
    expect(next).not.toHaveProperty('assistantEnabled');
    expect(sessionCapabilities()).not.toHaveProperty('assistantEnabled');
  });

  it('preserves an explicit businessEstate===true capability', () => {
    const next = syncSessionCapabilities({
      sessionCapabilities: { demoMode: false, businessEstate: true },
    });

    expect(next).toEqual({ demoMode: false, businessEstate: true });
    expect(sessionCapabilities()).toEqual({ demoMode: false, businessEstate: true });
  });

  // Same strict-equality gate as demoMode: only the boolean `true` survives.
  it.each([
    ['number 1', 1],
    ['string "true"', 'true'],
    ['null', null],
    ['undefined', undefined],
  ])('coerces a non-boolean businessEstate (%s) to false', (_label, businessEstate) => {
    const next = syncSessionCapabilities({
      sessionCapabilities: {
        demoMode: false,
        businessEstate: businessEstate as unknown as boolean,
      },
    });

    expect(next).toEqual({ demoMode: false, businessEstate: false });
  });
});
