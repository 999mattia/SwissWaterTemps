import { describe, expect, it } from 'vitest';
import { dailyRows, medianStep, nearestIndex, niceTicks, splitGaps, yDomain, type TimedValue } from './chart';

const H = 3600_000;
const series = (values: number[], step = H, start = Date.UTC(2026, 6, 14)): TimedValue[] =>
  values.map((value, i) => ({ time: start + i * step, value }));

describe('niceTicks', () => {
  it('uses clean steps', () => {
    expect(niceTicks(15.5, 21.5)).toEqual([16, 18, 20]);
    expect(niceTicks(17, 19)).toEqual([17, 17.5, 18, 18.5, 19]);
    expect(niceTicks(0, 100)).toEqual([0, 50, 100]);
  });

  it('handles a flat range', () => {
    expect(niceTicks(5, 5)).toEqual([5]);
  });
});

describe('yDomain', () => {
  it('pads and snaps to half degrees', () => {
    expect(yDomain([16.2, 21.7])).toEqual([15.5, 22.5]);
  });

  it('keeps at least two degrees for flat data', () => {
    const [lo, hi] = yDomain([18, 18.1]);
    expect(hi - lo).toBeGreaterThanOrEqual(2);
  });
});

describe('splitGaps', () => {
  it('breaks the line where data is missing', () => {
    const pts = [...series([1, 2, 3]), ...series([4, 5], H, Date.UTC(2026, 6, 15))];
    expect(splitGaps(pts, 3 * H).map((s) => s.length)).toEqual([3, 2]);
    expect(splitGaps([], H)).toEqual([]);
  });
});

describe('medianStep', () => {
  it('ignores the odd gap', () => {
    const pts = [...series([1, 2, 3, 4]), { time: Date.UTC(2026, 6, 20), value: 5 }];
    expect(medianStep(pts)).toBe(H);
  });
});

describe('nearestIndex', () => {
  const pts = series([1, 2, 3, 4]);
  it('finds the closest point', () => {
    expect(nearestIndex(pts, pts[1].time + 0.4 * H)).toBe(1);
    expect(nearestIndex(pts, pts[1].time + 0.6 * H)).toBe(2);
    expect(nearestIndex(pts, 0)).toBe(0);
    expect(nearestIndex(pts, Infinity)).toBe(3);
    expect(nearestIndex([], 0)).toBe(-1);
  });
});

describe('dailyRows', () => {
  it('groups by local day, newest first', () => {
    const start = new Date(2026, 6, 14, 0, 0).getTime();
    const rows = dailyRows(series([10, 14, 12, 20], 12 * H, start));
    expect(rows).toEqual([
      { day: new Date(2026, 6, 15).getTime(), min: 12, max: 20 },
      { day: new Date(2026, 6, 14).getTime(), min: 10, max: 14 },
    ]);
  });
});
