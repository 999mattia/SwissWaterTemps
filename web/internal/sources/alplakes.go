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
// model that Eawag runs for 131 lakes and publishes through the Alplakes API
// (https://www.alplakes.eawag.ch). The model runs once a day, so the series are
// refreshed every TTL. The API allows only short bursts (about 20 requests),
// so the lakes are fetched one by one, Pace apart, in the background: Fetch
// never waits for it and takes the current values from the cached series.
// Its values are fallbacks: a fresh measurement on the same lake or basin
// hides them.
type Alplakes struct {
	Client  *http.Client
	BaseURL string
	TTL     time.Duration
	// Pace is the pause between two requests, Backoff the wait before
	// retrying a lake that hit the rate limit.
	Pace, Backoff time.Duration
	now           func() time.Time

	mu         sync.Mutex
	series     map[string][]station.Point
	started    time.Time // of the last refresh
	refreshing bool
	err        error // of the last complete refresh
	wg         sync.WaitGroup
}

func NewAlplakes(client *http.Client) *Alplakes {
	return &Alplakes{
		Client:  client,
		BaseURL: "https://alplakes-api.eawag.ch",
		TTL:     3 * time.Hour,
		Pace:    2 * time.Second,
		Backoff: 15 * time.Second,
		now:     time.Now,
	}
}

func (a *Alplakes) ID() string   { return "alplakes" }
func (a *Alplakes) Name() string { return "Alplakes (Eawag)" }
func (a *Alplakes) URL() string  { return "https://www.alplakes.eawag.ch" }

const (
	// alplakesHistory is fetched once to backfill the history; later refreshes
	// only need the last two days.
	alplakesHistory = 7 * 24 * time.Hour
	alplakesRecent  = 48 * time.Hour
)

// Fetch starts a background refresh when one is due and returns the lakes'
// current values from the cached series, with the last refresh's error. While
// the first refresh hasn't returned any lake yet, it returns neither stations
// nor an error, so the store keeps the stations it restored at startup.
func (a *Alplakes) Fetch(ctx context.Context) ([]station.Station, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	if !a.refreshing && (a.started.IsZero() || now.Sub(a.started) >= a.TTL) {
		a.refreshing, a.started = true, now
		a.wg.Add(1)
		go a.refresh(ctx, now)
	}
	stations := lakeStations(a.series, now)
	if len(stations) == 0 && a.err == nil && a.refreshing {
		return nil, nil
	}
	if len(stations) == 0 && a.err == nil {
		return nil, errors.New("no current lake values")
	}
	return stations, a.err
}

func (a *Alplakes) refresh(ctx context.Context, now time.Time) {
	defer a.wg.Done()
	err := a.fetchAll(ctx, now)
	if err != nil {
		slog.Warn("refreshing Alplakes failed", "error", err)
	}
	a.mu.Lock()
	a.refreshing, a.err = false, err
	a.mu.Unlock()
}

func (a *Alplakes) fetchAll(ctx context.Context, now time.Time) error {
	lakes, err := a.fetchMetadata(ctx, now)
	if err != nil {
		return err
	}

	a.mu.Lock()
	from := now.Add(-alplakesHistory)
	if a.series != nil {
		from = now.Add(-alplakesRecent)
	} else {
		a.series = map[string][]station.Point{}
	}
	a.mu.Unlock()
	// Up to the next refresh, so the current value keeps advancing in between.
	to := now.Add(a.TTL)

	var errs []error
	for _, key := range lakes {
		if err := sleep(ctx, a.Pace); err != nil {
			return err
		}
		points, err := a.fetchSeries(ctx, key, from, to)
		if isTooManyRequests(err) {
			if err := sleep(ctx, a.Backoff); err != nil {
				return err
			}
			points, err = a.fetchSeries(ctx, key, from, to)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
			continue // the lake keeps its previous series
		}
		a.mu.Lock()
		a.series[key] = points
		a.mu.Unlock()
	}
	return partialError(a.ID(), errs, len(lakes))
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isTooManyRequests(err error) bool {
	var se *statusError
	return errors.As(err, &se) && se.code == http.StatusTooManyRequests
}

// fetchMetadata lists the lakes the model currently runs and that we know (see
// alplakesLakes); lakes whose simulation ended are skipped.
func (a *Alplakes) fetchMetadata(ctx context.Context, now time.Time) ([]string, error) {
	body, err := get(ctx, a.Client, a.BaseURL+"/simulations/1d/metadata")
	if err != nil {
		return nil, err
	}
	var models []struct {
		Model string `json:"model"`
		Lakes []struct {
			Name    string `json:"name"`
			EndDate string `json:"end_date"`
		} `json:"lakes"`
	}
	if err := json.Unmarshal(body, &models); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}

	var lakes []string
	for _, m := range models {
		if m.Model != "simstrat" {
			continue
		}
		for _, l := range m.Lakes {
			if _, known := alplakesLakes[l.Name]; !known {
				continue
			}
			end, err := time.Parse(time.DateOnly, l.EndDate)
			if err != nil || end.Before(now.Add(-72*time.Hour)) {
				continue
			}
			lakes = append(lakes, l.Name)
		}
	}
	if len(lakes) == 0 {
		return nil, errors.New("metadata lists no known lakes")
	}
	return lakes, nil
}

func (a *Alplakes) fetchSeries(ctx context.Context, lake string, from, to time.Time) ([]station.Point, error) {
	const layout = "200601021504"
	u := fmt.Sprintf("%s/simulations/1d/point/simstrat/%s/%s/%s/0?variables=T",
		a.BaseURL, url.PathEscape(lake), from.UTC().Format(layout), to.UTC().Format(layout))
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
		return nil, fmt.Errorf("decode series: %w", err)
	}
	temps := res.Variables["T"].Data
	if len(temps) != len(res.Time) {
		return nil, fmt.Errorf("series has %d times but %d temperatures", len(res.Time), len(temps))
	}

	points := make([]station.Point, 0, len(temps))
	for i, ts := range res.Time {
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil || temps[i] == nil || math.IsNaN(*temps[i]) || *temps[i] < -2 || *temps[i] > 40 {
			continue
		}
		points = append(points, station.Point{Time: t, Value: *temps[i]})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Time.Before(points[j].Time) })
	return points, nil
}

// lakeStations turns the cached series into stations as of now: the latest
// value at or before now is the current one.
func lakeStations(series map[string][]station.Point, now time.Time) []station.Station {
	stations := make([]station.Station, 0, len(series))
	for key, points := range series {
		info := alplakesLakes[key]
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

		s := station.Station{
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
			HydroKey:    lakeGauge(info.name),
			Fallback:    true,
			Modelled:    true,
			Recent:      recent,
		}
		if info.waterBody == "" {
			s.WaterBody = info.name
		}
		if info.basin {
			s.Basin = key
		}
		if info.rmse > 0 {
			s.ModelError = ptr(info.rmse)
		}
		stations = append(stations, s)
	}
	return stations
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
