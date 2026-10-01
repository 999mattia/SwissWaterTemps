package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Zurich reads the Zürichsee water temperature measured every 10 minutes by
// the city of Zürich's water police (Wasserschutzpolizei) at its two weather
// stations, through the tecdottir API over the city's open data.
type Zurich struct {
	Client  *http.Client
	BaseURL string
}

func NewZurich(client *http.Client) *Zurich {
	return &Zurich{Client: client, BaseURL: "https://tecdottir.metaodi.ch"}
}

func (z *Zurich) ID() string   { return "wapo" }
func (z *Zurich) Name() string { return "Wasserschutzpolizei Zürich" }
func (z *Zurich) URL() string {
	return "https://data.stadt-zuerich.ch/dataset/sid_wapo_wetterstationen"
}

var zurichStations = []struct {
	key, name string
	lat, lon  float64
}{
	{"tiefenbrunnen", "Zürichsee – Tiefenbrunnen", 47.3483, 8.5596},
	{"mythenquai", "Zürichsee – Mythenquai", 47.3570, 8.5363},
}

func (z *Zurich) Fetch(ctx context.Context) ([]station.Station, error) {
	var out []station.Station
	var errs []error
	for _, zs := range zurichStations {
		// The last 24 hours, newest first: the current value plus history to backfill.
		u := fmt.Sprintf("%s/measurements/%s?sort=%s&limit=144", z.BaseURL, zs.key, url.QueryEscape("timestamp_cet desc"))
		body, err := get(ctx, z.Client, u)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		s, err := parseZurich(body)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", zs.key, err))
			continue
		}
		s.ID = "wapo-" + zs.key
		s.Name = zs.name
		s.Lat, s.Lon = ptr(zs.lat), ptr(zs.lon)
		out = append(out, s)
	}
	return out, errors.Join(errs...)
}

type zurichValue struct {
	Value  json.RawMessage `json:"value"`
	Status string          `json:"status"`
}

func parseZurich(body []byte) (station.Station, error) {
	var res struct {
		OK     bool `json:"ok"`
		Result []struct {
			Values map[string]zurichValue `json:"values"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return station.Station{}, fmt.Errorf("decode: %w", err)
	}

	s := station.Station{WaterBody: "Zürichsee", Kind: station.Lake, Source: "wapo", HydroKey: lakeGauge("Zürichsee")}
	// Newest first; Recent is stored oldest first like every other series.
	for i := len(res.Result) - 1; i >= 0; i-- {
		v := res.Result[i].Values
		var ts string
		var temp *float64
		if json.Unmarshal(v["timestamp_cet"].Value, &ts) != nil || v["water_temperature"].Status != "ok" ||
			json.Unmarshal(v["water_temperature"].Value, &temp) != nil || temp == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			continue
		}
		s.Recent = append(s.Recent, station.Point{Time: t, Value: *temp})
	}
	if len(s.Recent) == 0 {
		return station.Station{}, errors.New("no valid water temperature")
	}
	last := s.Recent[len(s.Recent)-1]
	s.Temperature, s.MeasuredAt = last.Value, &last.Time
	return s, nil
}
