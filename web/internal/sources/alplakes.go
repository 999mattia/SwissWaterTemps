package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Alplakes reads modelled lake surface temperatures from the Simstrat 1D lake
// model that Eawag runs operationally and publishes through the Alplakes API
// (https://www.alplakes.eawag.ch, data under Apache 2.0). The model is updated
// about once a day and includes a forecast, so the series are fetched at most
// every TTL and the current value is derived from the cached series on every
// call.
type Alplakes struct {
	Client  *http.Client
	BaseURL string
	TTL     time.Duration
	now     func() time.Time

	mu        sync.Mutex
	series    map[string][]station.Point
	fetchedAt time.Time
}

func NewAlplakes(client *http.Client) *Alplakes {
	return &Alplakes{
		Client:  client,
		BaseURL: "https://alplakes-api.eawag.ch",
		TTL:     time.Hour,
		now:     time.Now,
	}
}

func (a *Alplakes) ID() string   { return "alplakes" }
func (a *Alplakes) Name() string { return "Alplakes (Eawag)" }
func (a *Alplakes) URL() string  { return "https://www.alplakes.eawag.ch" }

const (
	alplakesHistory     = 7 * 24 * time.Hour
	alplakesForecast    = 5 * 24 * time.Hour
	alplakesConcurrency = 4
)

// Fetch returns the lakes' current values. If refreshing the series fails but
// older series are cached, it returns stations from the cache together with
// the error.
func (a *Alplakes) Fetch(ctx context.Context) ([]station.Station, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	var err error
	if a.series == nil || now.Sub(a.fetchedAt) >= a.TTL {
		err = a.refresh(ctx, now)
	}
	if len(a.series) == 0 {
		if err == nil {
			err = errors.New("Alplakes returned no lake data")
		}
		return nil, err
	}
	return lakeStations(a.series, now), err
}

func (a *Alplakes) refresh(ctx context.Context, now time.Time) error {
	lakes, err := a.fetchMetadata(ctx, now)
	if err != nil {
		return err
	}

	start := now.Add(-alplakesHistory).UTC()
	type result struct {
		key    string
		points []station.Point
		err    error
	}
	jobs := make(chan simstratLake)
	results := make(chan result)
	var wg sync.WaitGroup
	for range alplakesConcurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for lake := range jobs {
				from := start
				if lake.startDate.After(from) {
					from = lake.startDate
				}
				to := now.Add(alplakesForecast).UTC()
				if end := lake.endDate.Add(23 * time.Hour); end.Before(to) {
					to = end
				}
				points, err := a.fetchSeries(ctx, lake.key, from, to)
				results <- result{lake.key, points, err}
			}
		}()
	}
	go func() {
		for _, l := range lakes {
			jobs <- l
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	series := make(map[string][]station.Point, len(lakes))
	var failed []string
	var lastErr error
	for r := range results {
		if r.err != nil {
			failed = append(failed, r.key)
			lastErr = r.err
			slog.Warn("fetching Alplakes lake failed", "lake", r.key, "error", r.err)
			continue
		}
		series[r.key] = r.points
	}

	// Keep the previous series of lakes that failed this time.
	for _, key := range failed {
		if old, ok := a.series[key]; ok {
			series[key] = old
		}
	}
	a.series = series
	a.fetchedAt = now

	if len(failed) > 0 {
		sort.Strings(failed)
		return fmt.Errorf("%d of %d lakes failed (%v): %w", len(failed), len(lakes), failed, lastErr)
	}
	return nil
}

type simstratLake struct {
	key                string
	startDate, endDate time.Time
}

// fetchMetadata lists the lakes that Simstrat currently simulates and that we
// know (see alplakesLakes); lakes whose simulation ended are skipped.
func (a *Alplakes) fetchMetadata(ctx context.Context, now time.Time) ([]simstratLake, error) {
	body, err := get(ctx, a.Client, a.BaseURL+"/simulations/1d/metadata")
	if err != nil {
		return nil, err
	}
	var models []struct {
		Model string `json:"model"`
		Lakes []struct {
			Name      string `json:"name"`
			StartDate string `json:"start_date"`
			EndDate   string `json:"end_date"`
		} `json:"lakes"`
	}
	if err := json.Unmarshal(body, &models); err != nil {
		return nil, fmt.Errorf("decode Alplakes metadata: %w", err)
	}

	var lakes []simstratLake
	for _, m := range models {
		if m.Model != "simstrat" {
			continue
		}
		for _, l := range m.Lakes {
			if _, known := alplakesLakes[l.Name]; !known {
				slog.Debug("skipping unknown Alplakes lake", "lake", l.Name)
				continue
			}
			start, err1 := time.Parse(time.DateOnly, l.StartDate)
			end, err2 := time.Parse(time.DateOnly, l.EndDate)
			if err1 != nil || err2 != nil || end.Before(now.Add(-72*time.Hour)) {
				continue
			}
			lakes = append(lakes, simstratLake{l.Name, start, end})
		}
	}
	if len(lakes) == 0 {
		return nil, errors.New("Alplakes metadata lists no known lakes")
	}
	return lakes, nil
}

func (a *Alplakes) fetchSeries(ctx context.Context, lake string, from, to time.Time) ([]station.Point, error) {
	const layout = "200601021504"
	u := fmt.Sprintf("%s/simulations/1d/point/simstrat/%s/%s/%s/0?variables=T",
		a.BaseURL, url.PathEscape(lake), from.Format(layout), to.Format(layout))
	body, err := get(ctx, a.Client, u)
	if err != nil {
		return nil, err
	}
	return parseAlplakesPoint(body)
}

func parseAlplakesPoint(body []byte) ([]station.Point, error) {
	var res struct {
		Time      []string `json:"time"`
		Variables map[string]struct {
			Data []*float64 `json:"data"`
		} `json:"variables"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("decode Alplakes series: %w", err)
	}
	temps := res.Variables["T"].Data
	if len(temps) != len(res.Time) {
		return nil, fmt.Errorf("Alplakes series has %d times but %d temperatures", len(res.Time), len(temps))
	}

	points := make([]station.Point, 0, len(temps))
	for i, ts := range res.Time {
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil || temps[i] == nil {
			continue
		}
		points = append(points, station.Point{Time: t, Value: *temps[i]})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Time.Before(points[j].Time) })
	return points, nil
}

// lakeStations turns the cached series into stations as of now: the latest
// value at or before now is the current one, later values are the forecast.
func lakeStations(series map[string][]station.Point, now time.Time) []station.Station {
	stations := make([]station.Station, 0, len(series))
	for key, points := range series {
		info := alplakesLakes[key]
		// Index of the first point after now.
		split := sort.Search(len(points), func(i int) bool { return points[i].Time.After(now) })
		if split == 0 {
			continue
		}
		current := points[split-1]
		if now.Sub(current.Time) > 48*time.Hour {
			continue // the simulation stopped; don't show old values as current
		}

		recent := points[:split]
		lo, hi := current.Value, current.Value
		for _, p := range recent {
			if current.Time.Sub(p.Time) < 24*time.Hour {
				lo, hi = min(lo, p.Value), max(hi, p.Value)
			}
		}

		stations = append(stations, station.Station{
			ID:          "alplakes-" + key,
			Name:        info.name,
			WaterBody:   info.waterBody,
			Kind:        station.Lake,
			Temperature: round1(current.Value),
			MeasuredAt:  ptr(current.Time),
			Min24h:      ptr(round1(lo)),
			Max24h:      ptr(round1(hi)),
			Lat:         ptr(info.lat),
			Lon:         ptr(info.lon),
			Source:      "alplakes",
			Modelled:    true,
			Recent:      recent,
			Forecast:    points[split:],
		})
	}
	return stations
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
