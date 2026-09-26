<script lang="ts">
  import { fetchHistory } from '../lib/api';
  import { dailyRows, toTimed, valueAt } from '../lib/chart';
  import type { Translator } from '../lib/i18n';
  import { isStale, temperatureHue } from '../lib/stations';
  import { load, save } from '../lib/storage';
  import type { Source, Station, StationHistory } from '../lib/types';
  import TemperatureChart from './TemperatureChart.svelte';
  import Trend from './Trend.svelte';

  interface Props {
    id: string;
    /** The station from the list, shown while the history loads. */
    initial?: Station;
    sources: Source[];
    tr: Translator;
    now: number;
    favourite: boolean;
    onToggleFavourite: (id: string) => void;
    onBack: () => void;
  }

  let { id, initial, sources, tr, now, favourite, onToggleFavourite, onBack }: Props = $props();

  const RANGES = [
    [7, 'days7'],
    [30, 'days30'],
    [365, 'days365'],
  ] as const;

  let days = $state<number>(load('swt:historyDays', 7));
  let data = $state<StationHistory | null>(null);
  let failed = $state(false);
  let notFound = $state(false);

  $effect(() => save('swt:historyDays', days));

  $effect(() => {
    const controller = new AbortController();
    failed = false;
    notFound = false;
    fetchHistory(id, days, controller.signal)
      .then((d) => (data = d))
      .catch((err: Error) => {
        if (err.name === 'AbortError') return;
        if (err.message === 'HTTP 404') notFound = true;
        else failed = true;
      });
    return () => controller.abort();
  });

  const station = $derived(data?.station.id === id ? data.station : initial);
  const history = $derived(data?.station.id === id ? toTimed(data.history) : []);
  const forecast = $derived(data?.station.id === id ? toTimed(data.forecast) : []);
  // A 5-day forecast is a sliver on a year-long chart, so it is only drawn up to 30 days.
  const chartForecast = $derived(days <= 30 ? forecast : []);
  const source = $derived(sources.find((s) => s.id === station?.source));
  const stale = $derived(station ? isStale(station, now) : false);
  const tomorrow = $derived(valueAt(forecast, now + 86_400_000));
  const rows = $derived(dailyRows(history));

  $effect(() => {
    if (station) document.title = `${station.name} – SwissWaterTemps`;
    return () => {
      document.title = 'SwissWaterTemps';
    };
  });
</script>

<article class="detail">
  <button class="back" onclick={onBack}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M15 5l-7 7 7 7" /></svg>
    {tr.t('back')}
  </button>

  {#if notFound && !station}
    <p class="empty">{tr.t('notFound')}</p>
  {:else if station}
    <header>
      <div class="title">
        <h2>{station.name}</h2>
        <p class="meta">
          {tr.t(station.kind)}
          {#if station.waterBody && !station.name.startsWith(station.waterBody)}· {station.waterBody}{/if}
        </p>
      </div>
      <button
        class="fav"
        class:active={favourite}
        aria-pressed={favourite}
        aria-label={tr.t(favourite ? 'removeFavourite' : 'addFavourite')}
        onclick={() => onToggleFavourite(station.id)}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z" stroke-linejoin="round" />
        </svg>
      </button>
    </header>

    <div class="hero">
      <div class="temp" class:stale style:--hue={temperatureHue(station.temperature)}>
        {tr.temp(station.temperature)}<span class="unit">°C</span>
      </div>
      <dl class="facts">
        {#if station.change24h != null}
          <div><dt class="visually-hidden">24 h</dt><dd><Trend change={station.change24h} {tr} /></dd></div>
        {/if}
        {#if station.measuredAt}
          <div>
            <dt>{tr.t(station.modelled ? 'modelled' : 'measured')}</dt>
            <dd><time datetime={station.measuredAt}>{tr.ago(station.measuredAt, now)}</time>
              {#if stale}<span class="badge">{tr.t('stale')}</span>{/if}</dd>
          </div>
        {/if}
        {#if station.min24h != null && station.max24h != null}
          <div><dt>{tr.t('range24h')}</dt><dd>{tr.temp(station.min24h)}–{tr.temp(station.max24h)}°</dd></div>
        {/if}
        {#if tomorrow != null}
          <div><dt>{tr.t('tomorrow')}</dt><dd>≈ {tr.temp(tomorrow)}°</dd></div>
        {/if}
      </dl>
    </div>

    {#if station.modelled}
      <p class="hint">{tr.t('modelledHint')}</p>
    {/if}

    <section class="card">
      <div class="ranges" role="radiogroup" aria-label={tr.t('chartLabel', { name: station.name })}>
        {#each RANGES as [value, key] (value)}
          <button role="radio" aria-checked={days === value} class:active={days === value} onclick={() => (days = value)}>
            {tr.t(key)}
          </button>
        {/each}
      </div>

      {#if failed}
        <p class="empty">{tr.t('historyError')}</p>
      {:else if !data || data.station.id !== id}
        <div class="placeholder" aria-hidden="true"></div>
      {:else if history.length + forecast.length < 2}
        <p class="empty">{tr.t('noHistory')}</p>
      {:else}
        <TemperatureChart {history} forecast={chartForecast} {tr} {now} label={tr.t('chartLabel', { name: station.name })} />

        {#if rows.length > 0}
          <details>
            <summary>{tr.t('showTable')}</summary>
            <table>
              <thead>
                <tr><th scope="col">{tr.t('date')}</th><th scope="col">{tr.t('min')}</th><th scope="col">{tr.t('max')}</th></tr>
              </thead>
              <tbody>
                {#each rows as row (row.day)}
                  <tr>
                    <td>{tr.fullDate(new Date(row.day))}</td>
                    <td>{tr.temp(row.min)}°</td>
                    <td>{tr.temp(row.max)}°</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </details>
        {/if}
      {/if}
    </section>

    {#if source}
      <p class="source">
        {tr.t('source')}: <a href={source.url} target="_blank" rel="noopener">{source.name}</a>
      </p>
    {/if}
  {:else if failed}
    <p class="empty">{tr.t('historyError')}</p>
  {:else}
    <div class="placeholder" aria-hidden="true"></div>
  {/if}
</article>

<style>
  .back {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    font: inherit;
    font-weight: 600;
    color: var(--accent);
    background: none;
    border: 0;
    padding: 0.4rem 0.4rem 0.4rem 0;
    margin-bottom: 0.5rem;
    cursor: pointer;
    min-height: 44px;
  }

  .back svg {
    width: 20px;
    height: 20px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2.2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.5rem;
  }

  h2 {
    margin: 0;
    font-size: 1.5rem;
    line-height: 1.2;
    overflow-wrap: anywhere;
  }

  .meta {
    margin: 0.2rem 0 0;
    color: var(--muted);
    font-size: 0.9rem;
  }

  .hero {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 1rem 1.5rem;
    margin: 1rem 0;
  }

  .temp {
    background: hsl(var(--hue) 85% 93%);
    color: hsl(var(--hue) 75% 26%);
    font-weight: 700;
    font-size: 2.6rem;
    line-height: 1;
    font-variant-numeric: tabular-nums;
    padding: 0.6rem 0.9rem;
    border-radius: 16px;
  }

  .unit {
    font-size: 0.5em;
    margin-left: 0.1em;
  }

  @media (prefers-color-scheme: dark) {
    .temp {
      background: hsl(var(--hue) 45% 20%);
      color: hsl(var(--hue) 85% 78%);
    }
  }

  .temp.stale {
    background: var(--chip);
    color: var(--muted);
  }

  .facts {
    display: grid;
    grid-template-columns: auto auto;
    gap: 0.25rem 1.5rem;
    margin: 0;
    font-size: 0.9rem;
  }

  .facts div {
    display: contents;
  }

  dt {
    color: var(--muted);
  }

  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }

  /* The trend row has no visible label; let it span both columns. */
  .facts div:first-child:has(.visually-hidden) dd {
    grid-column: 1 / -1;
  }

  .badge {
    color: var(--warning);
    font-weight: 600;
    margin-left: 0.3rem;
  }

  .hint {
    font-size: 0.82rem;
    color: var(--muted);
    margin: -0.25rem 0 1rem;
  }

  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 0.75rem 0.75rem 0.5rem;
  }

  .ranges {
    display: inline-flex;
    background: var(--chip);
    border-radius: 10px;
    padding: 3px;
    margin-bottom: 0.75rem;
  }

  .ranges button {
    font: inherit;
    font-size: 0.85rem;
    border: 0;
    background: none;
    color: var(--text);
    padding: 0.3rem 0.7rem;
    border-radius: 8px;
    cursor: pointer;
    min-height: 32px;
  }

  .ranges button.active {
    background: var(--surface);
    box-shadow: 0 1px 3px rgb(0 0 0 / 0.12);
    font-weight: 600;
  }

  .placeholder {
    height: 220px;
    border-radius: 10px;
    background: linear-gradient(90deg, var(--chip), var(--bg), var(--chip));
    background-size: 200% 100%;
    animation: shimmer 1.4s ease-in-out infinite;
  }

  @keyframes shimmer {
    to {
      background-position: -200% 0;
    }
  }

  .empty {
    color: var(--muted);
    text-align: center;
    padding: 2rem 1rem;
    margin: 0;
  }

  details {
    margin-top: 0.5rem;
    font-size: 0.85rem;
  }

  summary {
    cursor: pointer;
    color: var(--muted);
    padding: 0.4rem 0;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-variant-numeric: tabular-nums;
  }

  th,
  td {
    text-align: right;
    padding: 0.3rem 0.4rem;
    border-bottom: 1px solid var(--border);
  }

  th:first-child,
  td:first-child {
    text-align: left;
  }

  th {
    color: var(--muted);
    font-weight: 600;
  }

  .source {
    font-size: 0.82rem;
    color: var(--muted);
  }

  .source a {
    color: inherit;
  }

  .fav {
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    flex: none;
    padding: 0;
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    border-radius: 50%;
  }

  .fav svg {
    width: 24px;
    height: 24px;
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
</style>
