package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Hydro reads BAFU's discharge and water level stations, the GeoJSON behind
// hydrodaten.admin.ch's "Aktuelle Abflüsse und Wasserstände" map. It isn't a
// temperature source: the store attaches its readings to stations by key.
type Hydro struct {
	Client  *http.Client
	DataURL string
}

func NewHydro(client *http.Client) *Hydro {
	return &Hydro{
		Client:  client,
		DataURL: "https://www.hydrodaten.admin.ch/web-hydro-maps/hydro_sensor_pq.geojson",
	}
}

// Fetch returns the latest reading of every station, keyed by BAFU station key.
func (h *Hydro) Fetch(ctx context.Context) (map[string]station.Hydro, error) {
	body, err := get(ctx, h.Client, h.DataURL)
	if err != nil {
		return nil, err
	}
	return parseHydro(body)
}

type hydroFile struct {
	Features []struct {
		Properties struct {
			Key            string `json:"key"`
			Icon           string `json:"icon"`
			LastMeasuredAt string `json:"last_measured_at"`
			// With their unit, e.g. "67 m³/s", "19 l/s", "501.82 m ü.M.".
			Discharge  string `json:"sensor_discharge_last_value"`
			WaterLevel string `json:"sensor_waterlevel_last_value"`
		} `json:"properties"`
	} `json:"features"`
}

// dangerIcon matches the map symbol of a station's current danger level,
// e.g. "river_level_3" or "lake_level_1" ("…_not_applicable" has none).
var dangerIcon = regexp.MustCompile(`_level_([1-5])$`)

func parseHydro(body []byte) (map[string]station.Hydro, error) {
	var file hydroFile
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("decode BAFU flow/level GeoJSON: %w", err)
	}

	out := make(map[string]station.Hydro, len(file.Features))
	for _, f := range file.Features {
		p := f.Properties
		if p.Key == "" {
			continue
		}
		var h station.Hydro
		if v, unit, ok := valueUnit(p.Discharge); ok {
			switch unit {
			case "m³/s":
				h.Discharge = &v
			case "l/s":
				h.Discharge = ptr(v / 1000)
			}
		}
		// Some gauges report a relative level ("0.05 m"), which says nothing on its own.
		if v, unit, ok := valueUnit(p.WaterLevel); ok && strings.Contains(unit, "ü.M.") {
			h.WaterLevel = &v
		}
		if m := dangerIcon.FindStringSubmatch(p.Icon); m != nil {
			n, _ := strconv.Atoi(m[1])
			h.DangerLevel = &n
		}
		if h.Discharge == nil && h.WaterLevel == nil {
			continue
		}
		h.MeasuredAt = parseTime(p.LastMeasuredAt)
		out[p.Key] = h
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("BAFU flow/level GeoJSON contains no usable stations")
	}
	return out, nil
}

// valueUnit splits "501.82 m ü.M." into 501.82 and "m ü.M.".
func valueUnit(s string) (float64, string, bool) {
	num, unit, _ := strings.Cut(strings.TrimSpace(s), " ")
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, "", false
	}
	return v, strings.TrimSpace(unit), true
}
