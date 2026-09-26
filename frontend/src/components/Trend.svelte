<script lang="ts">
  import type { Translator } from '../lib/i18n';

  interface Props {
    change: number;
    tr: Translator;
    /** Show only the arrow and the value, e.g. in list cards. */
    compact?: boolean;
  }

  let { change, tr, compact = false }: Props = $props();

  // Changes below 0.3 °C are within measurement noise and shown as steady.
  const direction = $derived(change >= 0.3 ? 'up' : change <= -0.3 ? 'down' : 'flat');
  const text = $derived(tr.t('change24h', { value: tr.signed(change) }));
</script>

<span class="trend {direction}" title={compact ? text : undefined} aria-label={compact ? text : undefined}>
  <svg viewBox="0 0 16 16" aria-hidden="true">
    {#if direction === 'up'}
      <path d="M4 10l4-4 4 4" />
    {:else if direction === 'down'}
      <path d="M4 6l4 4 4-4" />
    {:else}
      <path d="M3 8h10" />
    {/if}
  </svg>
  {compact ? `${tr.signed(change)}°` : text}
</span>

<style>
  .trend {
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  svg {
    width: 14px;
    height: 14px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
