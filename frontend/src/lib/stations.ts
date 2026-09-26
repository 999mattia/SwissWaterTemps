import type { KindFilter, Position, SortKey, Station } from './types';

/** Measurements older than this are shown as stale. */
export const STALE_AFTER_MS = 6 * 60 * 60 * 1000;

/** Lower-cases and strips accents, so "zurich" finds "Zürichsee". */
export function normalize(s: string): string {
  return s
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase();
}

/** Every whitespace-separated term has to occur in the name or water body. */
export function matchesQuery(station: Station, query: string): boolean {
  const terms = normalize(query).split(/\s+/).filter(Boolean);
  if (terms.length === 0) return true;
  const haystack = normalize(`${station.name} ${station.waterBody ?? ''}`);
  return terms.every((t) => haystack.includes(t));
}

/** Great-circle distance in kilometres. */
export function distanceKm(a: Position, b: Position): number {
  const rad = Math.PI / 180;
  const dLat = (b.lat - a.lat) * rad;
  const dLon = (b.lon - a.lon) * rad;
  const h =
    Math.sin(dLat / 2) ** 2 + Math.cos(a.lat * rad) * Math.cos(b.lat * rad) * Math.sin(dLon / 2) ** 2;
  return 2 * 6371 * Math.asin(Math.sqrt(h));
}

export function stationDistance(station: Station, position: Position | null): number | undefined {
  if (!position || station.lat == null || station.lon == null) return undefined;
  return distanceKm(position, { lat: station.lat, lon: station.lon });
}

const collator = new Intl.Collator('de-CH', { sensitivity: 'base', numeric: true });

export interface ListOptions {
  query: string;
  kind: KindFilter;
  sort: SortKey;
  position: Position | null;
}

export function filterAndSort(stations: Station[], opts: ListOptions): Station[] {
  const byName = (a: Station, b: Station) => collator.compare(a.name, b.name);
  const compare: Record<SortKey, (a: Station, b: Station) => number> = {
    name: byName,
    warmest: (a, b) => b.temperature - a.temperature || byName(a, b),
    coldest: (a, b) => a.temperature - b.temperature || byName(a, b),
    nearest: (a, b) => {
      // Stations without coordinates go last.
      const da = stationDistance(a, opts.position) ?? Infinity;
      const db = stationDistance(b, opts.position) ?? Infinity;
      return da - db || byName(a, b);
    },
  };

  return stations
    .filter((s) => (opts.kind === 'all' || s.kind === opts.kind) && matchesQuery(s, opts.query))
    .sort(compare[opts.sort]);
}

export function isStale(station: Station, now: number): boolean {
  if (!station.measuredAt) return false;
  return now - Date.parse(station.measuredAt) > STALE_AFTER_MS;
}

// Hue stops for water temperatures: cold blue, teal for "fresh", green for
// pleasant swimming water, then amber to red for warm.
const HUE_STOPS: [temp: number, hue: number][] = [
  [5, 225],
  [14, 200],
  [18, 172],
  [21, 135],
  [23, 42],
  [26, 12],
];

export function temperatureHue(t: number): number {
  if (t <= HUE_STOPS[0][0]) return HUE_STOPS[0][1];
  for (let i = 1; i < HUE_STOPS.length; i++) {
    const [t1, h1] = HUE_STOPS[i];
    if (t <= t1) {
      const [t0, h0] = HUE_STOPS[i - 1];
      return Math.round(h0 + ((t - t0) / (t1 - t0)) * (h1 - h0));
    }
  }
  return HUE_STOPS[HUE_STOPS.length - 1][1];
}

/** Solid marker colour for the map. */
export function temperatureColor(t: number): string {
  return `hsl(${temperatureHue(t)} 70% 42%)`;
}
