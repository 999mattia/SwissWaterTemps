import { describe, expect, it } from 'vitest';
import { distanceKm, filterAndSort, isStale, matchesQuery, temperatureHue } from './stations';
import type { Station } from './types';

const station = (s: Partial<Station>): Station => ({
  id: s.name ?? 'x',
  name: 'x',
  kind: 'lake',
  temperature: 15,
  source: 'test',
  ...s,
});

const stations = [
  station({ name: 'Zürichsee', temperature: 21, lat: 47.25, lon: 8.68 }),
  station({ name: 'Aare - Bern, Schönau', waterBody: 'Aare', kind: 'river', temperature: 17, lat: 46.95, lon: 7.44 }),
  station({ name: 'Lac Léman', temperature: 22 }),
  station({ name: 'Rhein - Basel', waterBody: 'Rhein', kind: 'river', temperature: 21, lat: 47.56, lon: 7.59 }),
];

describe('matchesQuery', () => {
  it('ignores case and accents', () => {
    expect(matchesQuery(stations[0], 'zurich')).toBe(true);
    expect(matchesQuery(stations[2], 'LEMAN')).toBe(true);
  });

  it('requires every term and searches the water body', () => {
    expect(matchesQuery(stations[1], 'aare bern')).toBe(true);
    expect(matchesQuery(stations[1], 'aare basel')).toBe(false);
    expect(matchesQuery(stations[3], 'rhein')).toBe(true);
  });

  it('matches everything for an empty query', () => {
    expect(matchesQuery(stations[0], '   ')).toBe(true);
  });
});

describe('filterAndSort', () => {
  const names = (list: Station[]) => list.map((s) => s.name);
  const base = { query: '', kind: 'all' as const, position: null };

  it('sorts by name with Swiss collation', () => {
    expect(names(filterAndSort(stations, { ...base, sort: 'name' }))).toEqual([
      'Aare - Bern, Schönau',
      'Lac Léman',
      'Rhein - Basel',
      'Zürichsee',
    ]);
  });

  it('sorts by temperature and breaks ties by name', () => {
    expect(names(filterAndSort(stations, { ...base, sort: 'warmest' }))).toEqual([
      'Lac Léman',
      'Rhein - Basel',
      'Zürichsee',
      'Aare - Bern, Schönau',
    ]);
    expect(filterAndSort(stations, { ...base, sort: 'coldest' })[0].name).toBe('Aare - Bern, Schönau');
  });

  it('filters by kind', () => {
    expect(names(filterAndSort(stations, { ...base, kind: 'river', sort: 'name' }))).toEqual([
      'Aare - Bern, Schönau',
      'Rhein - Basel',
    ]);
  });

  it('sorts by distance with unknown locations last', () => {
    const olten = { lat: 47.35, lon: 7.9 };
    expect(names(filterAndSort(stations, { ...base, sort: 'nearest', position: olten }))).toEqual([
      'Rhein - Basel',
      'Aare - Bern, Schönau',
      'Zürichsee',
      'Lac Léman',
    ]);
  });

  it('does not mutate its input', () => {
    const copy = [...stations];
    filterAndSort(stations, { ...base, sort: 'warmest' });
    expect(stations).toEqual(copy);
  });
});

describe('distanceKm', () => {
  it('is roughly right for Bern to Zürich', () => {
    expect(distanceKm({ lat: 46.948, lon: 7.447 }, { lat: 47.376, lon: 8.541 })).toBeCloseTo(95, -1);
  });
});

describe('isStale', () => {
  const now = Date.parse('2026-07-14T12:00:00Z');
  it('flags old measurements only', () => {
    expect(isStale(station({ measuredAt: '2026-07-14T11:00:00Z' }), now)).toBe(false);
    expect(isStale(station({ measuredAt: '2026-07-13T11:00:00Z' }), now)).toBe(true);
    expect(isStale(station({}), now)).toBe(false);
  });
});

describe('temperatureHue', () => {
  it('interpolates between stops and clamps at both ends', () => {
    expect(temperatureHue(0)).toBe(225);
    expect(temperatureHue(5)).toBe(225);
    expect(temperatureHue(16)).toBe(186);
    expect(temperatureHue(21)).toBe(135);
    expect(temperatureHue(40)).toBe(12);
  });

  it('gets warmer (lower hue) as the temperature rises', () => {
    for (let t = 5; t < 26; t += 0.5) expect(temperatureHue(t + 0.5)).toBeLessThanOrEqual(temperatureHue(t));
  });
});
