<script lang="ts">
  import { medianStep, nearestIndex, niceTicks, splitGaps, yDomain, type TimedValue } from '../lib/chart';
  import type { Translator } from '../lib/i18n';

  interface Props {
    history: TimedValue[];
    forecast: TimedValue[];
    tr: Translator;
    now: number;
    label: string;
  }

  let { history, forecast, tr, now, label }: Props = $props();

  const HEIGHT = 220;
  const M = { top: 16, right: 44, bottom: 26, left: 34 };
  const DAY = 86_400_000;

  let width = $state(0);
  let hover = $state<{ point: TimedValue; forecast: boolean } | null>(null);

  const all = $derived([...history, ...forecast]);
  const innerW = $derived(Math.max(0, width - M.left - M.right));
  const innerH = HEIGHT - M.top - M.bottom;

  const xMin = $derived(all.length ? all[0].time : now);
  const xMax = $derived(all.length ? all[all.length - 1].time : now);
  const [yMin, yMax] = $derived(yDomain(all.map((p) => p.value)));

  const x = (t: number) => M.left + (xMax === xMin ? innerW / 2 : ((t - xMin) / (xMax - xMin)) * innerW);
  const y = (v: number) => M.top + (1 - (v - yMin) / (yMax - yMin)) * innerH;

  const path = (pts: TimedValue[]) =>
    pts.map((p, i) => `${i ? 'L' : 'M'}${x(p.time).toFixed(1)},${y(p.value).toFixed(1)}`).join('');

  // Missing data shows as gaps: anything over 3 typical steps (at least 6 h).
  const segments = $derived(splitGaps(history, Math.max(6 * 3600_000, 3 * medianStep(history))));
  const last = $derived(history.at(-1));
  // The forecast line starts at the latest known value so the two connect.
  const forecastLine = $derived(last && forecast.length ? [last, ...forecast] : forecast);

  const yTicks = $derived(niceTicks(yMin, yMax, 4));

  // X ticks at local midnights: daily for a week, weekly for a month, monthly beyond.
  const xTicks = $derived.by(() => {
    const span = xMax - xMin;
    const ticks: { time: number; label: string }[] = [];
    if (span <= 0) return ticks;
    const d = new Date(xMin);
    d.setHours(0, 0, 0, 0);
    if (span > 62 * DAY) {
      d.setDate(1);
      for (d.setMonth(d.getMonth() + 1); d.getTime() <= xMax; d.setMonth(d.getMonth() + 1)) {
        ticks.push({ time: d.getTime(), label: tr.month(d) });
      }
      return ticks.filter((_, i) => span < 200 * DAY || i % 2 === 0);
    }
    const stepDays = span > 12 * DAY ? 7 : span > 5 * DAY ? 2 : 1;
    for (d.setDate(d.getDate() + 1); d.getTime() <= xMax; d.setDate(d.getDate() + stepDays)) {
      ticks.push({ time: d.getTime(), label: stepDays === 7 ? tr.dayMonth(d) : tr.weekday(d) });
    }
    return ticks;
  });

  const showNow = $derived(forecast.length > 0 && now > xMin && now < xMax);

  function onPointer(e: PointerEvent) {
    const rect = (e.currentTarget as SVGElement).getBoundingClientRect();
    const t = xMin + ((e.clientX - rect.left - M.left) / innerW) * (xMax - xMin);
    const i = nearestIndex(all, t);
    hover = i < 0 ? null : { point: all[i], forecast: i >= history.length };
  }

  const tooltipLeft = $derived(hover ? Math.min(Math.max(x(hover.point.time), 70), width - 70) : 0);
</script>

<div class="chart" bind:clientWidth={width}>
  {#if width > 0 && all.length > 0}
    <svg
      {width}
      height={HEIGHT}
      role="img"
      aria-label={label}
      onpointermove={onPointer}
      onpointerdown={onPointer}
      onpointerleave={() => (hover = null)}
    >
      <!-- Grid and axes stay recessive: hairlines, muted text. -->
      {#each yTicks as tick (tick)}
        <line class="grid" x1={M.left} x2={width - M.right} y1={y(tick)} y2={y(tick)} />
        <text class="axis" x={M.left - 6} y={y(tick)} dy="0.32em" text-anchor="end">{tr.temp(tick).replace(/[.,]0$/, '')}°</text>
      {/each}
      {#each xTicks as tick (tick.time)}
        <text class="axis" x={x(tick.time)} y={HEIGHT - 8} text-anchor="middle">{tick.label}</text>
      {/each}

      {#if showNow}
        <line class="now" x1={x(now)} x2={x(now)} y1={M.top} y2={M.top + innerH} />
        <text class="axis" x={x(now)} y={M.top - 4} text-anchor="middle">{tr.t('now')}</text>
      {/if}

      {#each segments as seg, i (i)}
        {#if seg.length > 1}
          <path
            class="area"
            d="{path(seg)}L{x(seg[seg.length - 1].time).toFixed(1)},{M.top + innerH}L{x(seg[0].time).toFixed(1)},{M.top + innerH}Z"
          />
        {/if}
        <path class="line" d={path(seg)} />
      {/each}

      {#if forecastLine.length > 1}
        {@const end = forecastLine[forecastLine.length - 1]}
        <path class="line forecast" d={path(forecastLine)} />
        <text class="label" x={x(end.time) + 6} y={y(end.value)} dy="0.32em">{tr.t('forecast')}</text>
      {/if}

      {#if last}
        <circle class="dot" cx={x(last.time)} cy={y(last.value)} r="4.5" />
        {#if !forecast.length}
          <text class="label value" x={x(last.time) + 8} y={y(last.value)} dy="0.32em">{tr.temp(last.value)}°</text>
        {/if}
      {/if}

      {#if hover}
        <line class="crosshair" x1={x(hover.point.time)} x2={x(hover.point.time)} y1={M.top} y2={M.top + innerH} />
        <circle class="dot" cx={x(hover.point.time)} cy={y(hover.point.value)} r="4.5" />
      {/if}
    </svg>

    {#if hover}
      <div class="tooltip" style:left="{tooltipLeft}px" role="status">
        <strong>{tr.temp(hover.point.value)} °C</strong>
        <span>{tr.weekdayTime(new Date(hover.point.time))}{hover.forecast ? ` · ${tr.t('forecast')}` : ''}</span>
      </div>
    {/if}
  {/if}
</div>

<style>
  .chart {
    position: relative;
    width: 100%;
    min-height: 220px;
    touch-action: pan-y;
    user-select: none;
    -webkit-user-select: none;
  }

  svg {
    display: block;
    overflow: visible;
  }

  .grid {
    stroke: var(--border);
    stroke-width: 1;
    shape-rendering: crispEdges;
  }

  .axis {
    fill: var(--muted);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }

  .label {
    fill: var(--muted);
    font-size: 11px;
  }

  .label.value {
    fill: var(--text);
    font-size: 12px;
    font-weight: 600;
  }

  .line {
    fill: none;
    stroke: var(--accent);
    stroke-width: 2;
    stroke-linejoin: round;
    stroke-linecap: round;
  }

  .forecast {
    stroke-dasharray: 4 5;
  }

  .area {
    fill: var(--accent);
    opacity: 0.1;
  }

  .dot {
    fill: var(--accent);
    stroke: var(--surface);
    stroke-width: 2;
  }

  .now,
  .crosshair {
    stroke: var(--muted);
    stroke-width: 1;
    shape-rendering: crispEdges;
  }

  .now {
    opacity: 0.5;
  }

  /* Sits over the top of the plot, next to the crosshair. */
  .tooltip {
    position: absolute;
    top: 0;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 0.35rem 0.6rem;
    border-radius: 8px;
    background: var(--text);
    color: var(--bg);
    font-size: 0.78rem;
    line-height: 1.3;
    white-space: nowrap;
    pointer-events: none;
    box-shadow: 0 2px 8px rgb(0 0 0 / 0.15);
  }

  .tooltip strong {
    font-size: 0.95rem;
    font-variant-numeric: tabular-nums;
  }
</style>
