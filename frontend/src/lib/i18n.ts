export type Lang = 'de' | 'fr' | 'en';
export const LANGS: Lang[] = ['de', 'fr', 'en'];

const locales: Record<Lang, string> = { de: 'de-CH', fr: 'fr-CH', en: 'en-GB' };

const de = {
  tagline: 'Wassertemperaturen der Schweiz',
  search: 'See oder Fluss suchen …',
  clearSearch: 'Suche löschen',
  all: 'Alle',
  lakes: 'Seen',
  rivers: 'Flüsse',
  sortBy: 'Sortieren',
  sortName: 'Name',
  sortWarmest: 'Wärmste',
  sortColdest: 'Kälteste',
  sortNearest: 'In der Nähe',
  list: 'Liste',
  map: 'Karte',
  favourites: 'Favoriten',
  allStations: 'Alle Messstellen',
  addFavourite: 'Zu Favoriten hinzufügen',
  removeFavourite: 'Aus Favoriten entfernen',
  lake: 'See',
  river: 'Fluss',
  range24h: '24 h',
  stale: 'veraltet',
  noResults: 'Keine Messstellen gefunden.',
  loading: 'Lade Temperaturen …',
  loadError: 'Die Daten konnten nicht geladen werden.',
  offline: 'Offline – angezeigt werden die zuletzt geladenen Daten.',
  retry: 'Erneut versuchen',
  refresh: 'Aktualisieren',
  updated: 'Aktualisiert',
  sourceDown: '{source} ist zurzeit nicht erreichbar. Die Werte stammen vom {time}.',
  sourceNeverLoaded: '{source} ist zurzeit nicht erreichbar.',
  locationDenied: 'Standort nicht verfügbar – sortiert nach Name.',
  locating: 'Standort wird ermittelt …',
  kmAway: '{km} km',
  mapAttribution: 'Karte © swisstopo',
  disclaimer:
    'Die Daten gehören {sources}. Diese Website ist ein nicht-kommerzielles Schulprojekt; alle Angaben ohne Gewähr.',
  sourceCode: 'Quellcode auf GitHub',
  language: 'Sprache',
  installHint: 'Tipp: Über «Teilen» → «Zum Home-Bildschirm» als App installieren.',
  dismiss: 'Schliessen',
};

type Dict = typeof de;

const fr: Dict = {
  tagline: "Températures de l'eau en Suisse",
  search: 'Rechercher un lac ou une rivière …',
  clearSearch: 'Effacer la recherche',
  all: 'Tous',
  lakes: 'Lacs',
  rivers: 'Rivières',
  sortBy: 'Trier',
  sortName: 'Nom',
  sortWarmest: 'Plus chaud',
  sortColdest: 'Plus froid',
  sortNearest: 'À proximité',
  list: 'Liste',
  map: 'Carte',
  favourites: 'Favoris',
  allStations: 'Toutes les stations',
  addFavourite: 'Ajouter aux favoris',
  removeFavourite: 'Retirer des favoris',
  lake: 'Lac',
  river: 'Rivière',
  range24h: '24 h',
  stale: 'ancien',
  noResults: 'Aucune station trouvée.',
  loading: 'Chargement des températures …',
  loadError: 'Impossible de charger les données.',
  offline: 'Hors ligne – dernières données chargées affichées.',
  retry: 'Réessayer',
  refresh: 'Actualiser',
  updated: 'Mis à jour',
  sourceDown: "{source} n'est pas joignable. Les valeurs datent du {time}.",
  sourceNeverLoaded: "{source} n'est pas joignable.",
  locationDenied: 'Position indisponible – tri par nom.',
  locating: 'Recherche de la position …',
  kmAway: '{km} km',
  mapAttribution: 'Carte © swisstopo',
  disclaimer:
    "Les données appartiennent à {sources}. Ce site est un projet scolaire non commercial ; informations sans garantie.",
  sourceCode: 'Code source sur GitHub',
  language: 'Langue',
  installHint: "Astuce : « Partager » → « Sur l'écran d'accueil » pour l'installer comme app.",
  dismiss: 'Fermer',
};

const en: Dict = {
  tagline: 'Water temperatures in Switzerland',
  search: 'Search a lake or river …',
  clearSearch: 'Clear search',
  all: 'All',
  lakes: 'Lakes',
  rivers: 'Rivers',
  sortBy: 'Sort',
  sortName: 'Name',
  sortWarmest: 'Warmest',
  sortColdest: 'Coldest',
  sortNearest: 'Nearby',
  list: 'List',
  map: 'Map',
  favourites: 'Favourites',
  allStations: 'All stations',
  addFavourite: 'Add to favourites',
  removeFavourite: 'Remove from favourites',
  lake: 'Lake',
  river: 'River',
  range24h: '24 h',
  stale: 'outdated',
  noResults: 'No stations found.',
  loading: 'Loading temperatures …',
  loadError: 'The data could not be loaded.',
  offline: 'Offline – showing the last loaded data.',
  retry: 'Try again',
  refresh: 'Refresh',
  updated: 'Updated',
  sourceDown: '{source} is currently unreachable. Values are from {time}.',
  sourceNeverLoaded: '{source} is currently unreachable.',
  locationDenied: 'Location unavailable – sorted by name.',
  locating: 'Finding your location …',
  kmAway: '{km} km',
  mapAttribution: 'Map © swisstopo',
  disclaimer: 'The data belongs to {sources}. This is a non-commercial school project; no guarantee of accuracy.',
  sourceCode: 'Source code on GitHub',
  language: 'Language',
  installHint: 'Tip: use “Share” → “Add to Home Screen” to install it as an app.',
  dismiss: 'Close',
};

const dicts: Record<Lang, Dict> = { de, fr, en };

export type Key = keyof Dict;

export function detectLang(): Lang {
  for (const l of navigator.languages ?? [navigator.language]) {
    const short = l.slice(0, 2) as Lang;
    if (LANGS.includes(short)) return short;
  }
  return 'de';
}

export function translator(lang: Lang) {
  const dict = dicts[lang];
  const locale = locales[lang];
  const temp = new Intl.NumberFormat(locale, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  const km = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 });
  const rel = new Intl.RelativeTimeFormat(locale, { numeric: 'auto', style: 'short' });
  const dateTime = new Intl.DateTimeFormat(locale, { dateStyle: 'short', timeStyle: 'short' });

  return {
    locale,
    t(key: Key, vars: Record<string, string> = {}): string {
      return dict[key].replace(/\{(\w+)\}/g, (_, name) => vars[name] ?? '');
    },
    temp: (v: number) => temp.format(v),
    km: (v: number) => km.format(v),
    dateTime: (iso: string) => dateTime.format(new Date(iso)),
    /** "vor 12 Min.", "vor 3 Std." … */
    ago(iso: string, now: number): string {
      const seconds = Math.round((Date.parse(iso) - now) / 1000);
      const abs = Math.abs(seconds);
      if (abs < 60) return rel.format(0, 'minute');
      if (abs < 3600) return rel.format(Math.round(seconds / 60), 'minute');
      if (abs < 86400) return rel.format(Math.round(seconds / 3600), 'hour');
      return rel.format(Math.round(seconds / 86400), 'day');
    },
  };
}

export type Translator = ReturnType<typeof translator>;
