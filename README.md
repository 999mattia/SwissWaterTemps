# SwissWaterTemps

Check the temperatures of lakes and rivers across Switzerland. Works as an
installable app (PWA): on an iPhone open the site in Safari, tap **Share →
Add to Home Screen**.

## Features

-   Lakes and rivers with the time of each measurement, the 24 h min/max range and
    the change over the last 24 hours
-   A page per station with a 7-day, 30-day or 1-year chart
-   River flow, water level and BAFU's flood danger level, where BAFU has a
    gauge on the same water (rivers by station, lakes by their level gauge)
-   Instant search (ignores accents, so "zurich" finds "Zürichsee")
-   Filter by lakes or rivers, sort by name, warmest, coldest or nearest
-   Favourites pinned to the top
-   Map view on swisstopo tiles
-   German, French and English
-   Works offline with the last loaded data
-   If a source is down, its last good values stay visible with a notice

## Architecture

```
frontend/   Svelte 5 + Vite single-page app and PWA (service worker, manifest, icons)
web/        Go server; embeds the built frontend into a single binary
  internal/sources/  one fetcher per data source (BAFU, boot24, HiKa Wetter),
                     plus BAFU's flow/level stations
  internal/store/    polls all sources in the background and caches the result
  internal/history/  SQLite database with the readings and the last state of each source
  internal/server/   JSON API and static file serving
garmin/     Garmin Connect IQ watch app
```

The server fetches all sources in parallel every 10 minutes. Requests only
read the in-memory cache, so they never wait on the upstream sites. If a
source fails, the server keeps that source's last good data and marks the
source as not OK.

### Data sources

| Source | What | Notes |
| --- | --- | --- |
| [BAFU](https://www.hydrodaten.admin.ch/de/seen-und-fluesse/messstationen-temperatur) | River temperatures, measured | GeoJSON, updated about every 10 minutes |
| [boot24](https://www.boot24.ch/chde/service/temperaturen/) | Lake temperatures | Scraped from the HTML table; approximate lake positions come from `web/internal/sources/lakes.go` |
| [HiKa Wetter](https://hikawetter.ch) | Wohlensee, measured | JSON |
| [BAFU](https://www.hydrodaten.admin.ch/de/seen-und-fluesse/messstationen-zustand) | Flow, water level and danger level | GeoJSON; attached to the temperature stations by station key (lakes via `lakeGauges` in `lakes.go`). If it fails, the last readings stay and temperatures are unaffected |

### History

Every reading is stored in hourly buckets in SQLite (`$DATA_DIR/swisswatertemps.db`)
and kept for two years. The database also holds the last data of every source,
so after a restart the app shows data immediately instead of waiting for the
first fetch. If the directory isn't writable the app still runs, just without
history, trends and restored state.

### API

| Endpoint | Description |
| --- | --- |
| `GET /api/v1/stations` | All stations with measurement time, 24 h range and change, coordinates, `hydro` (flow in m³/s, level in m a.s.l., danger level 1–5) where available, plus the status of every source |
| `GET /api/v1/stations/{id}/history?days=7` | One station with its stored readings (hourly; daily averages beyond 31 days, at most 730 days) |
| `GET /api/temperatures` | Legacy format used by the Garmin app: `{lakeTemperatures, riverTemperatures}` with `{name, temperature}` |
| `GET /healthz` | Liveness check |

### Configuration

| Variable | Default | |
| --- | --- | --- |
| `PORT` | `3000` | HTTP port |
| `REFRESH_INTERVAL` | `10m` | How often the sources are fetched (Go duration) |
| `DATA_DIR` | `data` (`/data` in Docker) | Where the history database is stored |

## Development

Requirements: Go 1.26+ and Node.js 22+.

```sh
# Backend on :3000
cd web && go run .

# Frontend dev server with hot reload on :5173, proxies /api to :3000
cd frontend && npm install && npm run dev
```

Checks:

```sh
cd web && go vet ./... && go test ./...
cd frontend && npm run check && npm test
```

Production build (the frontend is written to `web/internal/ui/dist` and embedded
into the binary):

```sh
cd frontend && npm run build
cd ../web && go build -o swisswatertemps .
```

Or with Docker (built from the repository root):

```sh
docker build -t swisswatertemps .
docker run -p 3000:3000 -v swisswatertemps-data:/data swisswatertemps
```

The app icons in `frontend/public` are generated from `favicon.svg` with
`npm run generate-pwa-assets`.

## Disclaimer

The data belongs to the BAFU (https://www.hydrodaten.admin.ch/de/seen-und-fluesse/messstationen-temperatur), boot24 (https://www.boot24.ch/chde/service/temperaturen/) and HiKa Wetter (https://hikawetter.ch).
Map tiles © swisstopo. This is a non-profit website, used for educational purposes.

## Garmin Watch App Installation

Required:

-   Garmin Connect IQ SDK (https://formulae.brew.sh/cask/connectiq)
-   Visual Studio Code (https://formulae.brew.sh/cask/visual-studio-code) with MonkeyC Extension
-   If you are on a Mac: Android File Transfer (https://formulae.brew.sh/cask/android-file-transfer#default)

1. Clone this repo
2. Open the project in VS Code
3. Open Command Palette and execute "Monkey C: Generate a Developer Key"
4. Save the developer key to desired location
5. Check in manifest.xml if your device is supoorted. If not, add it
6. Open Command Palette and execute "Monkey C: Build for Device"
7. Select your desired device
8. Save the output to desired folder
9. Connect your watch to your pc
10. Access the watch in the file explorer and open the GARMIN/Apps folder (on Mac use Android File Explorer)
11. Copy the .prg file from the output the the GARMIN/Apps folder
