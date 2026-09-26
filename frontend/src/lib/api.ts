import type { Snapshot, StationHistory } from './types';

export async function fetchSnapshot(signal?: AbortSignal): Promise<Snapshot> {
  const res = await fetch('/api/v1/stations', { signal, headers: { Accept: 'application/json' } });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return (await res.json()) as Snapshot;
}

export async function fetchHistory(id: string, days: number, signal?: AbortSignal): Promise<StationHistory> {
  const res = await fetch(`/api/v1/stations/${encodeURIComponent(id)}/history?days=${days}`, {
    signal,
    headers: { Accept: 'application/json' },
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return (await res.json()) as StationHistory;
}
