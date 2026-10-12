package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Vorarlberg reads the Bodensee at Bregenz from the state of Vorarlberg's
// hydrography (VOWIS), measured every 10 minutes: in the harbour and at the
// Strandbad Mili, half a metre below the surface.
type Vorarlberg struct {
	Client  *http.Client
	DataURL string
}

func NewVorarlberg(client *http.Client) *Vorarlberg {
	return &Vorarlberg{Client: client, DataURL: "https://vowis.vorarlberg.at/api/see"}
}

func (v *Vorarlberg) ID() string   { return "vowis" }
func (v *Vorarlberg) Name() string { return "Land Vorarlberg (VOWIS)" }
func (v *Vorarlberg) URL() string  { return "https://vowis.vorarlberg.at/stationswrapper/bodensee" }

func (v *Vorarlberg) Fetch(ctx context.Context) ([]station.Station, error) {
	body, err := get(ctx, v.Client, v.DataURL)
	if err != nil {
		return nil, err
	}
	return parseVorarlberg(body)
}

type vowisValue struct {
	Time  string   `json:"datum"`
	Value *float64 `json:"wert"`
}

func parseVorarlberg(body []byte) ([]station.Station, error) {
	var res []struct {
		Harbour vowisValue `json:"wTemperatur"`
		Mili    vowisValue `json:"wtMilli05"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(res) == 0 {
		return nil, errors.New("empty response")
	}

	var out []station.Station
	for _, p := range []struct {
		v        vowisValue
		id, name string
		lat, lon float64
	}{
		{res[0].Harbour, "vowis-bregenz-hafen", "Bodensee – Bregenz Hafen", 47.5050, 9.7415},
		{res[0].Mili, "vowis-bregenz-mili", "Bodensee – Bregenz Strandbad Mili", 47.5063, 9.7357},
	} {
		at, err := time.Parse(time.RFC3339, p.v.Time)
		if err != nil || p.v.Value == nil || *p.v.Value < -2 || *p.v.Value > 40 {
			continue
		}
		out = append(out, station.Station{
			ID: p.id, Name: p.name, WaterBody: "Bodensee", Kind: station.Lake,
			Temperature: math.Round(*p.v.Value*10) / 10, MeasuredAt: &at, Lat: ptr(p.lat), Lon: ptr(p.lon),
			Source: "vowis", HydroKey: lakeGauge("Bodensee"),
		})
	}
	if len(out) == 0 {
		return nil, errors.New("no water temperature")
	}
	return out, nil
}
