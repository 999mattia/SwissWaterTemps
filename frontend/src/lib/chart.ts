import type { Point } from './types';

export interface TimedValue {
  time: number; // epoch ms
  value: number;
}

/** How a metric's values are written: on the axis, next to the latest point, in the tooltip. */
export interface ValueFormat {
  tick: (v: number) => string;
  short: (v: number) => string;
  full: (v: number) => string;
  /** Smallest y range, and the step it snaps to (see yDomain). */
  minSpan: number;
  snap: number;
  /** Room for the axis labels, in px. */
  axisWidth: number;
}

export function toTimed(points: Point[]): TimedValue[] {
  return points.map((p) => ({ time: Date.parse(p.t), value: p.v }));
}

/** Clean, evenly spaced tick values (step 0.5, 1, 2 or 5 × 10ⁿ) covering [min, max]. */
export function niceTicks(min: number, max: number, target = 4): number[] {
  if (!(max > min)) return [min];
  const raw = (max - min) / target;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 5, 10].map((m) => m * mag).find((s) => s >= raw) ?? 10 * mag;
  const ticks: number[] = [];
  for (let v = Math.ceil(min / step) * step; v <= max + step * 1e-9; v += step) {
    ticks.push(Math.round(v / step) * step);
  }
  return ticks;
}

/**
 * Y range with some air around the data, at least minSpan wide (so small
 * wobbles don't fill the chart) and snapped outward to multiples of snap.
 * The defaults suit temperatures: 2 degrees, half degrees.
 */
export function yDomain(values: number[], minSpan = 2, snap = 0.5): [number, number] {
  let lo = Math.min(...values);
  let hi = Math.max(...values);
  if (hi - lo < minSpan) {
    const mid = (hi + lo) / 2;
    lo = mid - minSpan / 2;
    hi = mid + minSpan / 2;
  }
  const pad = (hi - lo) * 0.1;
  return [Math.floor((lo - pad) / snap) * snap, Math.ceil((hi + pad) / snap) * snap];
}

/**
 * Splits a series where consecutive points are further apart than maxGap,
 * so missing data shows as a gap instead of a straight line across it.
 */
export function splitGaps(points: TimedValue[], maxGap: number): TimedValue[][] {
  const segments: TimedValue[][] = [];
  let current: TimedValue[] = [];
  for (const p of points) {
    if (current.length > 0 && p.time - current[current.length - 1].time > maxGap) {
      segments.push(current);
      current = [];
    }
    current.push(p);
  }
  if (current.length > 0) segments.push(current);
  return segments;
}

/** Typical spacing of a series, used to decide what counts as a gap. */
export function medianStep(points: TimedValue[]): number {
  if (points.length < 2) return 0;
  const steps = points.slice(1).map((p, i) => p.time - points[i].time).sort((a, b) => a - b);
  return steps[Math.floor(steps.length / 2)];
}

/** Index of the point closest in time to t; points must be sorted by time. */
export function nearestIndex(points: TimedValue[], t: number): number {
  if (points.length === 0) return -1;
  let lo = 0;
  let hi = points.length - 1;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (points[mid].time < t) lo = mid + 1;
    else hi = mid;
  }
  if (lo > 0 && t - points[lo - 1].time < points[lo].time - t) return lo - 1;
  return lo;
}

export interface DayRow {
  day: number; // epoch ms of local midnight
  min: number;
  max: number;
}

/** Min and max per local calendar day, newest first. */
export function dailyRows(points: TimedValue[]): DayRow[] {
  const byDay = new Map<number, DayRow>();
  for (const p of points) {
    const d = new Date(p.time);
    d.setHours(0, 0, 0, 0);
    const key = d.getTime();
    const row = byDay.get(key);
    if (row) {
      row.min = Math.min(row.min, p.value);
      row.max = Math.max(row.max, p.value);
    } else {
      byDay.set(key, { day: key, min: p.value, max: p.value });
    }
  }
  return [...byDay.values()].sort((a, b) => b.day - a.day);
}
