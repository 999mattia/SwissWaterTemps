package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Wiewarm reads selected pools of wiewarm.ch, the bathing temperature
// platform. Most of its lake values are typed in by hand during the season;
// only pools with an automatic sensor are listed here.
type Wiewarm struct {
	Client  *http.Client
	BaseURL string
}

func NewWiewarm(client *http.Client) *Wiewarm {
	return &Wiewarm{Client: client, BaseURL: "https://www.wiewarm.ch/api/v1"}
}

func (w *Wiewarm) ID() string   { return "wiewarm" }
func (w *Wiewarm) Name() string { return "wiewarm.ch" }
func (w *Wiewarm) URL() string  { return "https://www.wiewarm.ch" }

var wiewarmPools = []struct {
	bad, becken     string
	id, name, water string
	lat, lon        float64
}{
	// Strandbad Thun: measured every 10 minutes.
	{"7", "23", "wiewarm-thun", "Thunersee – Strandbad Thun", "Thunersee", 46.7399, 7.6306},
}

func (w *Wiewarm) Fetch(ctx context.Context) ([]station.Station, error) {
	var out []station.Station
	var errs []error
	for _, p := range wiewarmPools {
		body, err := get(ctx, w.Client, w.BaseURL+"/bad.json/"+p.bad)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.id, err))
			continue
		}
		temp, at, err := parseWiewarm(body, p.becken)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.id, err))
			continue
		}
		out = append(out, station.Station{
			ID: p.id, Name: p.name, WaterBody: p.water, Kind: station.Lake,
			Temperature: temp, MeasuredAt: at, Lat: ptr(p.lat), Lon: ptr(p.lon),
			Source: "wiewarm", HydroKey: lakeGauge(p.water),
		})
	}
	return out, partialError(w.ID(), errs, len(wiewarmPools))
}

func parseWiewarm(body []byte, becken string) (float64, *time.Time, error) {
	var bad struct {
		Becken map[string]struct {
			ID   string `json:"beckenid"`
			Temp string `json:"temp"`
			Date string `json:"date"`
		} `json:"becken"`
	}
	if err := json.Unmarshal(body, &bad); err != nil {
		return 0, nil, fmt.Errorf("decode: %w", err)
	}
	for _, b := range bad.Becken {
		if b.ID != becken {
			continue
		}
		temp, err := strconv.ParseFloat(b.Temp, 64)
		if err != nil {
			return 0, nil, fmt.Errorf("temperature %q: %w", b.Temp, err)
		}
		// "2026-10-01 16:20:00" (local time, sometimes with fractional seconds)
		if len(b.Date) < 19 {
			return 0, nil, fmt.Errorf("date %q", b.Date)
		}
		at, err := time.ParseInLocation("2006-01-02 15:04:05", b.Date[:19], zurich)
		if err != nil {
			return 0, nil, err
		}
		return temp, &at, nil
	}
	return 0, nil, fmt.Errorf("pool %s not found", becken)
}
