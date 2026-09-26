<script lang="ts">
  import { onMount } from 'svelte';
  import StationCard from './components/StationCard.svelte';
  import { fetchSnapshot } from './lib/api';
  import { detectLang, LANGS, translator, type Lang } from './lib/i18n';
  import { filterAndSort, stationDistance } from './lib/stations';
  import { load, save } from './lib/storage';
  import type { KindFilter, Position, Snapshot, SortKey } from './lib/types';

  const REFRESH_MS = 5 * 60 * 1000;
  const GITHUB = 'https://github.com/999mattia/SwissWaterTemps';

  let lang = $state<Lang>(load('swt:lang', detectLang()));
  const tr = $derived(translator(lang));

  // The last snapshot is kept locally so the app paints instantly on launch.
  let snapshot = $state<Snapshot | null>(load('swt:snapshot', null));
  let loading = $state(false);
  let loadFailed = $state(false);
  let online = $state(navigator.onLine);
  let now = $state(Date.now());

  let query = $state('');
  let kind = $state<KindFilter>(load('swt:kind', 'all'));
  let sort = $state<SortKey>(load('swt:sort', 'name'));
  let view = $state<'list' | 'map'>(load('swt:view', 'list'));
  let favourites = $state<string[]>(load('swt:favourites', []));

  let position = $state<Position | null>(null);
  let locating = $state(false);
  let locationDenied = $state(false);

  const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent);
  const standalone =
    window.matchMedia('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true;
  let showInstallHint = $state(isIOS && !standalone && !load('swt:installHintDismissed', false));

  $effect(() => save('swt:lang', lang));
  $effect(() => save('swt:kind', kind));
  $effect(() => save('swt:view', view));
  $effect(() => save('swt:favourites', favourites));
  // "Nearby" needs a fresh location each launch, so it is not remembered.
  $effect(() => save('swt:sort', sort === 'nearest' ? 'name' : sort));
  $effect(() => {
    document.documentElement.lang = lang;
  });

  const favouriteSet = $derived(new Set(favourites));
  const visible = $derived(
    filterAndSort(snapshot?.stations ?? [], { query, kind, sort, position: sort === 'nearest' ? position : null }),
  );
  const favouriteStations = $derived(visible.filter((s) => favouriteSet.has(s.id)));
  const otherStations = $derived(visible.filter((s) => !favouriteSet.has(s.id)));
  const failingSources = $derived(snapshot?.sources.filter((s) => !s.ok) ?? []);
  const sourceLinks = $derived(snapshot?.sources ?? []);
  const disclaimer = $derived(tr.t('disclaimer', { sources: '\u0000' }).split('\u0000'));

  let inFlight: AbortController | undefined;

  async function refresh() {
    if (inFlight) return;
    inFlight = new AbortController();
    loading = true;
    try {
      const data = await fetchSnapshot(inFlight.signal);
      snapshot = data;
      loadFailed = false;
      save('swt:snapshot', data);
    } catch (err) {
      if ((err as Error).name !== 'AbortError') loadFailed = true;
    } finally {
      loading = false;
      inFlight = undefined;
      now = Date.now();
    }
  }

  function toggleFavourite(id: string) {
    favourites = favouriteSet.has(id) ? favourites.filter((f) => f !== id) : [...favourites, id];
  }

  function locate() {
    if (!('geolocation' in navigator)) {
      locationDenied = true;
      sort = 'name';
      return;
    }
    locating = true;
    locationDenied = false;
    navigator.geolocation.getCurrentPosition(
      (p) => {
        position = { lat: p.coords.latitude, lon: p.coords.longitude };
        locating = false;
      },
      () => {
        locating = false;
        locationDenied = true;
        sort = 'name';
      },
      { maximumAge: 10 * 60 * 1000, timeout: 15000 },
    );
  }

  $effect(() => {
    if (sort === 'nearest' && !position && !locating) locate();
  });

  function dismissInstallHint() {
    showInstallHint = false;
    save('swt:installHintDismissed', true);
  }

  onMount(() => {
    refresh();

    const clock = setInterval(() => (now = Date.now()), 30_000);
    const poll = setInterval(() => {
      if (document.visibilityState === 'visible') refresh();
    }, REFRESH_MS);

    // Home screen apps on iOS are resumed, not reloaded, so refresh when the
    // app comes back to the foreground with data older than a minute.
    const onVisible = () => {
      if (document.visibilityState !== 'visible') return;
      now = Date.now();
      const age = snapshot ? now - Date.parse(snapshot.updatedAt) : Infinity;
      if (age > 60_000) refresh();
    };
    const onOnline = () => {
      online = true;
      refresh();
    };
    const onOffline = () => (online = false);

    document.addEventListener('visibilitychange', onVisible);
    window.addEventListener('online', onOnline);
    window.addEventListener('offline', onOffline);
    return () => {
      clearInterval(clock);
      clearInterval(poll);
      inFlight?.abort();
      document.removeEventListener('visibilitychange', onVisible);
      window.removeEventListener('online', onOnline);
      window.removeEventListener('offline', onOffline);
    };
  });
</script>

<header class="top">
  <div class="bar">
    <div class="brand">
      <img src="/favicon.svg" alt="" width="32" height="32" />
      <div>
        <h1>SwissWaterTemps</h1>
        <p class="sub">
          {#if snapshot?.updatedAt && Date.parse(snapshot.updatedAt) > 0}
            {tr.t('updated')} {tr.ago(snapshot.updatedAt, now)}
          {:else}
            {tr.t('tagline')}
          {/if}
        </p>
      </div>
    </div>
    <div class="actions">
      <button
        class="icon-btn"
        class:spinning={loading}
        onclick={refresh}
        disabled={loading}
        aria-label={tr.t('refresh')}
        title={tr.t('refresh')}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M20 12a8 8 0 1 1-2.34-5.66M20 4v5h-5" />
        </svg>
      </button>
      <button
        class="icon-btn"
        onclick={() => (view = view === 'list' ? 'map' : 'list')}
        aria-label={tr.t(view === 'list' ? 'map' : 'list')}
        title={tr.t(view === 'list' ? 'map' : 'list')}
      >
        {#if view === 'list'}
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M9 4 3 6.5v13.5l6-2.5 6 2.5 6-2.5V4l-6 2.5z M9 4v13.5 M15 6.5V20" />
          </svg>
        {:else}
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 6h13M8 12h13M8 18h13M3.5 6h.01M3.5 12h.01M3.5 18h.01" /></svg>
        {/if}
      </button>
      <label class="lang">
        <span class="visually-hidden">{tr.t('language')}</span>
        <select bind:value={lang}>
          {#each LANGS as l (l)}
            <option value={l}>{l.toUpperCase()}</option>
          {/each}
        </select>
      </label>
    </div>
  </div>

  <div class="controls">
    <div class="search">
      <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
      <input
        type="search"
        bind:value={query}
        placeholder={tr.t('search')}
        aria-label={tr.t('search')}
        autocomplete="off"
        enterkeyhint="search"
      />
      {#if query}
        <button class="clear" onclick={() => (query = '')} aria-label={tr.t('clearSearch')}>×</button>
      {/if}
    </div>

    <div class="row">
      <div class="segmented" role="radiogroup" aria-label="{tr.t('lakes')} / {tr.t('rivers')}">
        {#each [['all', 'all'], ['lake', 'lakes'], ['river', 'rivers']] as const as [value, label] (value)}
          <button role="radio" aria-checked={kind === value} class:active={kind === value} onclick={() => (kind = value)}>
            {tr.t(label)}
          </button>
        {/each}
      </div>

      <label class="sort">
        <span class="visually-hidden">{tr.t('sortBy')}</span>
        <select bind:value={sort}>
          <option value="name">{tr.t('sortName')}</option>
          <option value="warmest">{tr.t('sortWarmest')}</option>
          <option value="coldest">{tr.t('sortColdest')}</option>
          <option value="nearest">{tr.t('sortNearest')}</option>
        </select>
      </label>

    </div>
  </div>
</header>

<main>
  {#if showInstallHint}
    <div class="notice info">
      <span>{tr.t('installHint')}</span>
      <button class="link" onclick={dismissInstallHint}>{tr.t('dismiss')}</button>
    </div>
  {/if}

  {#if !online && snapshot}
    <div class="notice">{tr.t('offline')}</div>
  {:else if loadFailed && snapshot}
    <div class="notice">
      <span>{tr.t('loadError')}</span>
      <button class="link" onclick={refresh}>{tr.t('retry')}</button>
    </div>
  {/if}

  {#each failingSources as source (source.id)}
    <div class="notice">
      {source.lastSuccess
        ? tr.t('sourceDown', { source: source.name, time: tr.dateTime(source.lastSuccess) })
        : tr.t('sourceNeverLoaded', { source: source.name })}
    </div>
  {/each}

  {#if locating}
    <div class="notice info">{tr.t('locating')}</div>
  {:else if locationDenied}
    <div class="notice">{tr.t('locationDenied')}</div>
  {/if}

  {#if !snapshot}
    <div class="empty">
      {#if loadFailed}
        <p>{tr.t('loadError')}</p>
        <button class="primary" onclick={refresh}>{tr.t('retry')}</button>
      {:else}
        <div class="loader" aria-hidden="true"></div>
        <p>{tr.t('loading')}</p>
      {/if}
    </div>
  {:else if view === 'map'}
    {#await import('./components/MapView.svelte') then { default: MapView }}
      <MapView stations={visible} {tr} {now} position={sort === 'nearest' ? position : null} />
    {/await}
    <p class="count">{visible.length} / {snapshot.stations.length}</p>
  {:else if visible.length === 0}
    <div class="empty"><p>{tr.t('noResults')}</p></div>
  {:else}
    {#if favouriteStations.length > 0}
      <section>
        <h2>{tr.t('favourites')}</h2>
        <ul class="list">
          {#each favouriteStations as station (station.id)}
            <StationCard
              {station}
              {tr}
              {now}
              distance={sort === 'nearest' ? stationDistance(station, position) : undefined}
              favourite
              onToggleFavourite={toggleFavourite}
            />
          {/each}
        </ul>
      </section>
    {/if}
    {#if otherStations.length > 0}
      <section>
        {#if favouriteStations.length > 0}<h2>{tr.t('allStations')}</h2>{/if}
        <ul class="list">
          {#each otherStations as station (station.id)}
            <StationCard
              {station}
              {tr}
              {now}
              distance={sort === 'nearest' ? stationDistance(station, position) : undefined}
              favourite={false}
              onToggleFavourite={toggleFavourite}
            />
          {/each}
        </ul>
      </section>
    {/if}
  {/if}
</main>

<footer>
  <p>
    {disclaimer[0]}{#each sourceLinks as source, i (source.id)}{#if i > 0}{i === sourceLinks.length - 1
          ? ' & '
          : ', '}{/if}<a href={source.url} target="_blank" rel="noopener">{source.name}</a>{/each}{disclaimer[1]}
  </p>
  <p><a href={GITHUB} target="_blank" rel="noopener">{tr.t('sourceCode')}</a></p>
</footer>

<style>
  .top {
    position: sticky;
    top: 0;
    z-index: 10;
    background: color-mix(in srgb, var(--bg) 88%, transparent);
    backdrop-filter: saturate(1.6) blur(14px);
    -webkit-backdrop-filter: saturate(1.6) blur(14px);
    border-bottom: 1px solid var(--border);
    padding: calc(env(safe-area-inset-top) + 0.6rem) var(--gutter) 0.7rem;
  }

  .bar,
  .controls,
  main,
  footer {
    max-width: 720px;
    margin-inline: auto;
  }

  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-width: 0;
  }

  .brand img {
    border-radius: 8px;
    flex: none;
  }

  .brand > div {
    min-width: 0;
  }

  h1 {
    margin: 0;
    font-size: 1.1rem;
    letter-spacing: -0.01em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .sub {
    margin: 0;
    font-size: 0.78rem;
    color: var(--muted);
  }

  .actions {
    flex: none;
    display: flex;
    align-items: center;
    gap: 0.25rem;
  }

  .icon-btn {
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    border: 0;
    border-radius: 50%;
    background: none;
    color: var(--text);
    cursor: pointer;
  }

  .icon-btn svg,
  .search svg {
    width: 20px;
    height: 20px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .icon-btn:disabled {
    cursor: default;
    opacity: 0.6;
  }

  .spinning svg {
    animation: spin 0.9s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  select {
    font: inherit;
    font-size: 0.9rem;
    color: var(--text);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 0.45rem 0.6rem;
    min-height: 38px;
  }

  .controls {
    margin-top: 0.6rem;
    display: grid;
    gap: 0.5rem;
  }

  .search {
    position: relative;
    display: flex;
    align-items: center;
  }

  .search svg {
    position: absolute;
    left: 0.75rem;
    color: var(--muted);
    pointer-events: none;
  }

  .search input {
    width: 100%;
    font: inherit;
    font-size: 16px; /* prevents iOS from zooming in on focus */
    color: var(--text);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 0.6rem 2.5rem 0.6rem 2.4rem;
    -webkit-appearance: none;
    appearance: none;
  }

  .search input::-webkit-search-cancel-button {
    display: none;
  }

  .search input:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
  }

  .clear {
    position: absolute;
    right: 0.3rem;
    width: 34px;
    height: 34px;
    border: 0;
    background: none;
    font-size: 1.4rem;
    line-height: 1;
    color: var(--muted);
    cursor: pointer;
  }

  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
  }

  .sort {
    flex: 1;
  }

  .sort select {
    width: 100%;
  }

  .segmented {
    display: inline-flex;
    background: var(--chip);
    border-radius: 10px;
    padding: 3px;
  }

  .segmented button {
    font: inherit;
    font-size: 0.88rem;
    border: 0;
    background: none;
    color: var(--text);
    padding: 0.35rem 0.7rem;
    border-radius: 8px;
    cursor: pointer;
    min-height: 32px;
  }

  .segmented button.active {
    background: var(--surface);
    box-shadow: 0 1px 3px rgb(0 0 0 / 0.12);
    font-weight: 600;
  }

  main {
    padding: 1rem var(--gutter) 0;
  }

  section + section {
    margin-top: 1.25rem;
  }

  h2 {
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    margin: 0 0 0.5rem 0.25rem;
  }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.5rem;
  }

  .notice {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    font-size: 0.88rem;
    padding: 0.6rem 0.85rem;
    margin-bottom: 0.6rem;
    border-radius: 10px;
    background: var(--warning-bg);
    color: var(--warning-text);
  }

  .notice.info {
    background: var(--info-bg);
    color: var(--info-text);
  }

  .link {
    font: inherit;
    font-weight: 600;
    background: none;
    border: 0;
    padding: 0.2rem;
    color: inherit;
    text-decoration: underline;
    cursor: pointer;
    white-space: nowrap;
  }

  .empty {
    text-align: center;
    padding: 3rem 1rem;
    color: var(--muted);
  }

  .primary {
    font: inherit;
    font-weight: 600;
    color: #fff;
    background: var(--accent);
    border: 0;
    border-radius: 10px;
    padding: 0.6rem 1.1rem;
    cursor: pointer;
  }

  .loader {
    width: 32px;
    height: 32px;
    margin: 0 auto 1rem;
    border: 3px solid var(--chip);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.9s linear infinite;
  }

  .count {
    text-align: right;
    font-size: 0.8rem;
    color: var(--muted);
    margin: 0.4rem 0 0;
  }

  footer {
    padding: 2rem var(--gutter) calc(env(safe-area-inset-bottom) + 1.5rem);
    font-size: 0.8rem;
    color: var(--muted);
    text-align: center;
  }

  footer a {
    color: inherit;
  }
</style>
