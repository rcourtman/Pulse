import { describe, expect, it } from 'vitest';
import { getGuestDrawerHistorySegments } from '../guestDrawerModel';

const point = (timestamp: number, value = 0) => ({ timestamp, value, min: value, max: value });

describe('stored History segments', () => {
  it('retains real points and both sides of each missing in-panel time', () => {
    const points = Object.freeze([point(1), point(2), point(4), point(6), point(7)]);
    const times = Object.freeze([1, 2, 3, 4, 5, 6, 7]);
    const segments = getGuestDrawerHistorySegments(points, times);
    expect(segments).toEqual([[points[0], points[1]], [points[2]], [points[3], points[4]]]);
    expect(segments.flat()).toEqual(points);
    expect(segments[1][0]).toBe(points[2]);
  });

  it('keeps duplicate-time readings in the same segment, without creating a new time', () => {
    const points = [point(1, 0), point(1, 10), point(2, 20), point(4, 30)];
    expect(getGuestDrawerHistorySegments(points, [1, 2, 3, 4])).toEqual([
      points.slice(0, 3),
      [points[3]],
    ]);
  });

  it('cannot guess gaps from elapsed time when all stored times match', () => {
    const points = [point(1), point(1_000_000)];
    expect(getGuestDrawerHistorySegments(points, [1, 1_000_000])).toEqual([points]);
  });

  it('keeps an empty series empty and never accepts a point outside the group times', () => {
    expect(getGuestDrawerHistorySegments([], [1, 2, 3])).toEqual([]);
    expect(getGuestDrawerHistorySegments([point(1)], [])).toEqual([]);
    expect(getGuestDrawerHistorySegments([point(1), point(2), point(3)], [1, 3])).toEqual([
      [point(1), point(3)],
    ]);
  });

  it('preserves every point at the existing request bound', () => {
    const times = Array.from({ length: 240 }, (_, index) => index);
    const points = times.filter((time) => time % 2 === 0).map((time) => point(time));
    const segments = getGuestDrawerHistorySegments(points, times);
    expect(segments).toHaveLength(120);
    expect(segments.every((segment) => segment.length === 1)).toBe(true);
    expect(segments.flat()).toEqual(points);
  });
});
