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
