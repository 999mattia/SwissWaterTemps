<script lang="ts">
  import L from 'leaflet';
  import 'leaflet/dist/leaflet.css';
  import { onMount, untrack } from 'svelte';
  import type { Translator } from '../lib/i18n';
  import { isStale, temperatureColor } from '../lib/stations';
  import type { Position, Station } from '../lib/types';

  interface Props {
    stations: Station[];
    tr: Translator;
    now: number;
    position: Position | null;
    onOpen: (id: string) => void;
  }

  let { stations, tr, now: current, position, onOpen }: Props = $props();

  const SWITZERLAND: L.LatLngBoundsExpression = [
    [45.8, 5.9],
    [47.85, 10.5],
  ];

  let container: HTMLDivElement;
  let map: L.Map | undefined = $state();
  let markers: L.LayerGroup | undefined;
  let you: L.CircleMarker | undefined;
  let fittedKey = '';

  onMount(() => {
    const m = L.map(container, { zoomControl: true, attributionControl: true }).fitBounds(SWITZERLAND);
    L.tileLayer(
      'https://wmts.geo.admin.ch/1.0.0/ch.swisstopo.pixelkarte-grau/default/current/3857/{z}/{x}/{y}.jpeg',
      {
        maxZoom: 18,
        attribution: '<a href="https://www.swisstopo.admin.ch/" target="_blank" rel="noopener">© swisstopo</a>',
      },
    ).addTo(m);
    markers = L.layerGroup().addTo(m);
    map = m;
    return () => m.remove();
  });

  function popup(s: Station): HTMLElement {
    const el = document.createElement('div');
    const name = document.createElement('strong');
    name.textContent = s.name;
    const temp = document.createElement('div');
    temp.textContent = `${tr.temp(s.temperature)} °C`;
    temp.className = 'popup-temp';
    el.append(name, temp);
    if (s.measuredAt) {
      const when = document.createElement('div');
      when.textContent = tr.ago(s.measuredAt, current);
      when.className = 'popup-meta';
      el.append(when);
    }
    const link = document.createElement('a');
    link.href = `/station/${encodeURIComponent(s.id)}`;
    link.textContent = `${tr.t('showDetails')} →`;
    link.className = 'popup-link';
    link.addEventListener('click', (e) => {
      e.preventDefault();
      onOpen(s.id);
    });
    el.append(link);
    return el;
  }

  // Redraw markers whenever the stations change, but only zoom when the set of
  // stations changes so the user's view is kept. The clock is not tracked:
  // redrawing on every tick would close open popups.
  $effect(() => {
    if (!map || !markers) return;
    const now = untrack(() => current);
    markers.clearLayers();
    const points: L.LatLngExpression[] = [];
    for (const s of stations) {
      if (s.lat == null || s.lon == null) continue;
      const color = isStale(s, now) ? '#8a96a3' : temperatureColor(s.temperature);
      L.circleMarker([s.lat, s.lon], {
        radius: 9,
        color: '#fff',
        weight: 2,
        fillColor: color,
        fillOpacity: 1,
      })
        .bindTooltip(`${tr.temp(s.temperature)}°`, { permanent: false, direction: 'top' })
        .bindPopup(() => popup(s))
        .addTo(markers);
      points.push([s.lat, s.lon]);
    }
    const key = stations.map((s) => s.id).join();
    if (points.length > 0 && key !== fittedKey) {
      fittedKey = key;
      map.fitBounds(L.latLngBounds(points), { padding: [30, 30], maxZoom: 12 });
    }
  });

  $effect(() => {
    if (!map) return;
    you?.remove();
    if (position) {
      you = L.circleMarker([position.lat, position.lon], {
        radius: 7,
        color: '#fff',
        weight: 3,
        fillColor: '#1a73e8',
        fillOpacity: 1,
      }).addTo(map);
    }
  });
</script>

<div class="map" bind:this={container}></div>

<style>
  .map {
    height: min(70vh, 640px);
    min-height: 360px;
    border-radius: 14px;
    overflow: hidden;
    border: 1px solid var(--border);
    z-index: 0;
  }

  .map :global(.popup-temp) {
    font-size: 1.2rem;
    font-weight: 700;
    margin-top: 0.2rem;
  }

  .map :global(.popup-link) {
    display: inline-block;
    margin-top: 0.3rem;
    font-weight: 600;
  }

  .map :global(.popup-meta) {
    color: #5b6773;
    font-size: 0.8rem;
  }
</style>
