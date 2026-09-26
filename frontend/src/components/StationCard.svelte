<script lang="ts">
  import type { Translator } from '../lib/i18n';
  import { isStale, temperatureHue } from '../lib/stations';
  import type { Station } from '../lib/types';
  import Trend from './Trend.svelte';

  interface Props {
    station: Station;
    tr: Translator;
    now: number;
    distance?: number;
    favourite: boolean;
    onToggleFavourite: (id: string) => void;
    onOpen: (id: string) => void;
  }

  let { station, tr, now, distance, favourite, onToggleFavourite, onOpen }: Props = $props();

  function open(e: MouseEvent) {
    // Let the browser handle new-tab clicks.
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    onOpen(station.id);
  }

  const stale = $derived(isStale(station, now));
  const showWaterBody = $derived(
    station.waterBody && !station.name.toLowerCase().startsWith(station.waterBody.toLowerCase()),
  );
</script>

<li class="card" class:stale>
  <div class="info">
    <h3 class="name">
      <a href="/station/{encodeURIComponent(station.id)}" onclick={open} aria-label={tr.t('details', { name: station.name })}
        >{station.name}</a
      >
    </h3>
    <p class="meta">
      <span class="kind">{tr.t(station.kind)}</span>
      {#if showWaterBody}<span>· {station.waterBody}</span>{/if}
      {#if distance != null}<span>· {tr.t('kmAway', { km: tr.km(distance) })}</span>{/if}
    </p>
    <p class="details">
      {#if station.change24h != null && !stale}
        <Trend change={station.change24h} {tr} compact />
      {/if}
      {#if station.measuredAt}
        <time datetime={station.measuredAt} title={tr.dateTime(station.measuredAt)}>
          {tr.ago(station.measuredAt, now)}
        </time>
        {#if stale}<span class="badge">{tr.t('stale')}</span>{/if}
      {/if}
      {#if station.min24h != null && station.max24h != null}
        <span class="range" title={tr.t('range24h')}>
          {tr.t('range24h')}: {tr.temp(station.min24h)}–{tr.temp(station.max24h)}°
        </span>
      {/if}
    </p>
  </div>

  <div class="temp" style:--hue={temperatureHue(station.temperature)}>
    {tr.temp(station.temperature)}<span class="unit">°C</span>
  </div>

  <button
    class="fav"
    class:active={favourite}
    aria-pressed={favourite}
    aria-label={tr.t(favourite ? 'removeFavourite' : 'addFavourite')}
    title={tr.t(favourite ? 'removeFavourite' : 'addFavourite')}
    onclick={() => onToggleFavourite(station.id)}
  >
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path
        d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z"
        stroke-linejoin="round"
      />
    </svg>
  </button>
</li>

<style>
  .card {
    position: relative;
    display: grid;
    grid-template-columns: 1fr auto auto;
    align-items: center;
    gap: 0.25rem 0.75rem;
    padding: 0.8rem 0.5rem 0.8rem 1rem;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 14px;
  }

  .info {
    min-width: 0;
  }

  .name {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  .name a {
    color: inherit;
    text-decoration: none;
  }

  /* The whole card opens the detail page; the star button sits above the link. */
  .name a::after {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: 14px;
  }

  .name a:focus-visible {
    outline: none;
  }

  .name a:focus-visible::after {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }

  .card:hover {
    border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
  }


  .meta,
  .details {
    margin: 0.15rem 0 0;
    font-size: 0.82rem;
    color: var(--muted);
    display: flex;
    flex-wrap: wrap;
    gap: 0 0.35rem;
  }

  .details {
    gap: 0 0.9rem;
  }

  .details:empty {
    display: none;
  }

  .range {
    font-variant-numeric: tabular-nums;
  }

  .badge {
    color: var(--warning);
    font-weight: 600;
  }

  .temp {
    background: hsl(var(--hue) 85% 93%);
    color: hsl(var(--hue) 75% 26%);
    font-weight: 700;
    font-size: 1.15rem;
    font-variant-numeric: tabular-nums;
    padding: 0.35rem 0.6rem;
    border-radius: 10px;
    white-space: nowrap;
    min-width: 5.2em;
    text-align: center;
  }

  .unit {
    font-size: 0.75em;
    font-weight: 600;
    margin-left: 0.1em;
    opacity: 0.9;
  }

  @media (prefers-color-scheme: dark) {
    .temp {
      background: hsl(var(--hue) 45% 20%);
      color: hsl(var(--hue) 85% 78%);
    }
  }

  .stale .temp {
    background: var(--chip);
    color: var(--muted);
  }

  .fav {
    position: relative;
    z-index: 1;
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    border-radius: 50%;
  }

  .fav svg {
    width: 22px;
    height: 22px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.8;
  }

  .fav.active {
    color: var(--star);
  }

  .fav.active svg {
    fill: currentColor;
  }

  .fav:focus-visible {
    outline: 2px solid var(--accent);
  }
</style>
