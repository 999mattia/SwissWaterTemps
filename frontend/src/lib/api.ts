import type { Snapshot } from './types';

export async function fetchSnapshot(signal?: AbortSignal): Promise<Snapshot> {
  const res = await fetch('/api/v1/stations', { signal, headers: { Accept: 'application/json' } });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return (await res.json()) as Snapshot;
}
