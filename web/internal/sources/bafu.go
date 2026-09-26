package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// BAFU reads the river temperature stations of the Federal Office for the
// Environment from the GeoJSON behind hydrodaten.admin.ch.
type BAFU struct {
	Client  *http.Client
	DataURL string
}

func NewBAFU(client *http.Client) *BAFU {
	return &BAFU{
		Client:  client,
		DataURL: "https://www.hydrodaten.admin.ch/web-hydro-maps/hydro_sensor_temperature.geojson",
	}
}

func (b *BAFU) ID() string   { return "bafu" }
func (b *BAFU) Name() string { return "BAFU" }
func (b *BAFU) URL() string {
	return "https://www.hydrodaten.admin.ch/de/seen-und-fluesse/messstationen-temperatur"
}

func (b *BAFU) Fetch(ctx context.Context) ([]station.Station, error) {
	body, err := get(ctx, b.Client, b.DataURL)
	if err != nil {
		return nil, err
	}
	return parseBAFU(body)
}

type bafuFile struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Label          string    `json:"label"`
			Key            string    `json:"key"`
			HydroBodyName  string    `json:"hydro_body_name"`
			LastValue      flexFloat `json:"last_value"`
			LastMeasuredAt string    `json:"last_measured_at"`
			Min24h         flexFloat `json:"min_24h"`
			Max24h         flexFloat `json:"max_24h"`
			HydroStationID flexInt   `json:"hydro_station_id"`
		} `json:"properties"`
	} `json:"features"`
}

func parseBAFU(body []byte) ([]station.Station, error) {
	var file bafuFile
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("decode BAFU GeoJSON: %w", err)
	}
	if len(file.Features) == 0 {
		return nil, fmt.Errorf("BAFU GeoJSON contains no features")
	}

	stations := make([]station.Station, 0, len(file.Features))
	for _, f := range file.Features {
		p := f.Properties
		if p.LastValue.v == nil || p.Label == "" {
			continue
		}

		id := p.Key
		if id == "" {
			id = strconv.Itoa(p.HydroStationID.v)
		}

		s := station.Station{
			ID:          "bafu-" + id,
			Name:        strings.TrimSpace(p.Label),
			WaterBody:   strings.TrimSpace(p.HydroBodyName),
			Kind:        station.River,
			Temperature: *p.LastValue.v,
			MeasuredAt:  parseTime(p.LastMeasuredAt),
			Min24h:      p.Min24h.v,
			Max24h:      p.Max24h.v,
			Source:      "bafu",
		}
		if c := f.Geometry.Coordinates; len(c) >= 2 {
			if lat, lon, ok := toWGS84(c[0], c[1]); ok {
				s.Lat, s.Lon = &lat, &lon
			}
		}
		stations = append(stations, s)
	}

	if len(stations) == 0 {
		return nil, fmt.Errorf("BAFU GeoJSON contains no usable stations")
	}
	return stations, nil
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		// Timestamps without an offset are local Swiss time.
		if t, err := time.ParseInLocation(layout, s, zurich); err == nil {
			return &t
		}
	}
	return nil
}

var zurich = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Zurich")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// toWGS84 accepts WGS84 (lon, lat), Swiss LV95 or LV03 coordinates and returns
// latitude and longitude, using swisstopo's approximate conversion formulas.
func toWGS84(x, y float64) (lat, lon float64, ok bool) {
	switch {
	case x >= -180 && x <= 180 && y >= -90 && y <= 90:
		return y, x, true
	case x > 2_000_000 && y > 1_000_000: // LV95 (E, N)
		x, y = x-2_600_000, y-1_200_000
	case x > 400_000 && y > 0 && y < 400_000: // LV03 (y, x)
		x, y = x-600_000, y-200_000
	default:
		return 0, 0, false
	}
	e, n := x/1e6, y/1e6
	lonP := 2.6779094 + 4.728982*e + 0.791484*e*n + 0.1306*e*n*n - 0.0436*e*e*e
	latP := 16.9023892 + 3.238272*n - 0.270978*e*e - 0.002528*n*n - 0.0447*e*e*n - 0.0140*n*n*n
	return latP * 100 / 36, lonP * 100 / 36, true
}

// flexFloat decodes numbers that may arrive as JSON numbers, strings or null.
type flexFloat struct{ v *float64 }

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	v, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return nil // treat unparsable values as missing rather than failing the whole file
	}
	f.v = &v
	return nil
}

type flexInt struct{ v int }

func (f *flexInt) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
	v, _ := strconv.Atoi(string(b))
	f.v = v
	return nil
}
