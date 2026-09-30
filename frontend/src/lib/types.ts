export type Kind = 'lake' | 'river';

export interface Station {
  id: string;
  name: string;
  waterBody?: string;
  kind: Kind;
  temperature: number;
  measuredAt?: string;
  min24h?: number;
  max24h?: number;
  lat?: number;
  lon?: number;
  source: string;
  /** Change compared to about 24 hours earlier, in °C. */
  change24h?: number;
  /** Flow and level at the BAFU gauge on the same water, if there is one. */
  hydro?: Hydro;
}

export interface Hydro {
  /** m³/s */
  discharge?: number;
  /** metres above sea level */
  waterLevel?: number;
  /** BAFU flood danger level, 1 (none or low) to 5 (very high) */
  dangerLevel?: number;
  measuredAt?: string;
}

export interface Point {
  /** ISO timestamp */
  t: string;
  v: number;
}

export interface StationHistory {
  station: Station;
  history: Point[];
}

export interface Source {
  id: string;
  name: string;
  url: string;
  ok: boolean;
  lastSuccess?: string;
  lastError?: string;
}

export interface Snapshot {
  updatedAt: string;
  sources: Source[];
  stations: Station[];
}

export type KindFilter = 'all' | Kind;
export type SortKey = 'name' | 'warmest' | 'coldest' | 'nearest';

export interface Position {
  lat: number;
  lon: number;
}
