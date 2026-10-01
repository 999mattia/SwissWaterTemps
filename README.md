# SwissWaterTemps

Check the temperatures of lakes and rivers across Switzerland. Works as an
installable app (PWA): on an iPhone open the site in Safari, tap **Share →
Add to Home Screen**.

## Features

-   Lakes and rivers with the time of each measurement, the 24 h min/max range and
    the change over the last 24 hours
-   Measured lake temperatures (Zürich water police, Eawag buoys, Strandbad
    Thun); boot24's approximate values fill in for the other lakes
-   A page per station with a 7-day, 30-day or 1-year chart of temperature, and
    of flow and water level where there's a BAFU gauge
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
  internal/sources/  one fetcher per data source (BAFU, boot24, HiKa Wetter,
                     Wasserschutzpolizei Zürich, Eawag Datalakes, wiewarm.ch),
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
| [Wasserschutzpolizei Zürich](https://data.stadt-zuerich.ch/dataset/sid_wapo_wetterstationen) | Zürichsee at Tiefenbrunnen and Mythenquai, measured every 10 minutes (published with about 2 hours' delay) | Open data via the tecdottir API; the last 24 h backfill the history |
| [Eawag Datalakes](https://www.datalakes-eawag.ch) | Surface temperature of research buoys: Genfersee, Vierwaldstättersee, Murtensee, Hallwilersee, Ägerisee, Lago di Lugano (2), Lago Maggiore | One JSON file per dataset and day; values with a quality flag are skipped. The datasets are listed in `datalakes.go` |
| [wiewarm.ch](https://www.wiewarm.ch) | Thunersee at Strandbad Thun (automatic sensor) | Most other wiewarm lake values are entered by hand, so only automatic pools are used (`wiewarm.go`) |
| [BAFU](https://www.hydrodaten.admin.ch/de/seen-und-fluesse/messstationen-zustand) | Flow, water level and danger level | GeoJSON; attached to the temperature stations by station key (lakes via `lakeGauges` in `lakes.go`). If it fails, the last readings stay and temperatures are unaffected |

A lake's boot24 entry is hidden while a measured station on the same lake
(matched by the lake's BAFU level gauge) has a value from the last 6 hours, and
comes back when the measurement stops.

### History

Every reading is stored in hourly buckets in SQLite (`$DATA_DIR/swisswatertemps.db`)
and kept for two years: temperature per station, flow and level per BAFU gauge
(only gauges a station uses). At that point the database stops growing at
roughly 120 MB. The database also holds the last data of every source,
so after a restart the app shows data immediately instead of waiting for the
first fetch. If the directory isn't writable the app still runs, just without
history, trends and restored state.

### API

| Endpoint | Description |
| --- | --- |
| `GET /api/v1/stations` | All stations with measurement time, 24 h range and change, coordinates, `hydro` (flow in m³/s, level in m a.s.l., danger level 1–5) where available, plus the status of every source |
| `GET /api/v1/stations/{id}/history?days=7` | One station with its stored readings (hourly; daily averages beyond 31 days, at most 730 days): `history` (temperature), `discharge` and `waterLevel` (empty without a gauge) |
| `GET /api/temperatures` | Legacy format used by the Garmin app: `{lakeTemperatures, riverTemperatures}` with `{name, temperature}` |
| `GET /healthz` | Liveness check |

### Configuration

| Variable | Default | |
| --- | --- | --- |
| `PORT` | `3000` | HTTP port |
| `REFRESH_INTERVAL` | `10m` | How often the sources are fetched (Go duration) |
| `DATA_DIR` | `data` (`/data` in Docker) | Where the history database is stored |

## Home Assistant

The API needs no key, so Home Assistant can read it with a `rest_command` and
trigger-based template sensors (one request for all stations). A package,
e.g. `packages/swisswatertemps.yaml`:

```yaml
rest_command:
  swisswatertemps:
    url: https://swt.mattiag.ch/api/v1/stations
    method: get

template:
  - triggers:
      - trigger: time_pattern
        minutes: "/10"
      - trigger: homeassistant
        event: start
    actions:
      - action: rest_command.swisswatertemps
        response_variable: r
        continue_on_error: true
      - variables:
          ok: "{{ r is defined and r.status == 200 and r.content is mapping }}"
          aare: "{{ (r.content.stations if ok else []) | selectattr('id', 'eq', 'bafu-2135') | first | default({}) }}"
    sensor:
      - name: Aare Bern water temperature
        unique_id: swt_aare_bern_temperature
        unit_of_measurement: °C
        device_class: temperature
        state_class: measurement
        availability: "{{ aare.temperature is number }}"
        state: "{{ aare.temperature }}"
        attributes:
          change_24h: "{{ aare.change24h | default(none) }}"
      - name: Aare Bern flow
        unique_id: swt_aare_bern_flow
        unit_of_measurement: m³/s
        device_class: volume_flow_rate
        state_class: measurement
        availability: "{{ (aare.hydro | default({})).discharge is number }}"
        state: "{{ aare.hydro.discharge }}"
```

Station ids are in `/api/v1/stations` (and in the URL of a station's page).

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

The measurements belong to their providers: the Federal Office for the
Environment BAFU (temperature, flow, level), boot24, HiKa Wetter, the
Wasserschutzpolizei of the city of Zürich, Eawag (Datalakes) and wiewarm.ch;
see the table above for links. Map tiles © swisstopo.

SwissWaterTemps is a private, non-commercial project. No guarantee of accuracy
or availability; swimming is at your own risk.

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
