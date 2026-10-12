package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Wiski reads the water temperature layer of a cantonal hydrology portal built
// on Kisters' WISKI web public. The layer is one JSON file with the latest
// value, time and position of every station, updated every 10–15 minutes.
type Wiski struct {
	Client   *http.Client
	LayerURL string

	id, name, site string
	// bafuPrefix marks the station numbers of BAFU stations ("U2044" in
	// Thurgau, "LH2104" in St. Gallen); those are already read from BAFU.
	bafuPrefix string
	// placeAfterSpace: names are "<water> <place>" ("Lauche Matzingen").
	placeAfterSpace bool
	// stations overrides name, water and kind by station number, for lakes
	// and names that can't be derived.
	stations map[string]wiskiStation
}

type wiskiStation struct {
	name, water string
	kind        station.Kind
}

// NewThurgau reads the portal of the cantons of Thurgau and Schaffhausen:
// Rhine, Thur and their tributaries, the Seerhein and the Bodensee at Altnau.
func NewThurgau(client *http.Client) *Wiski {
	return &Wiski{
		Client: client, LayerURL: "https://www.hydrodaten.tg.ch/data/internet/layers/30/index.json",
		id: "tg", name: "Hydrodaten Thurgau/Schaffhausen", site: "https://www.hydrodaten.tg.ch",
		bafuPrefix: "U", placeAfterSpace: true,
		stations: map[string]wiskiStation{
			"M1090": {"Bodensee – Altnau", "Bodensee", station.Lake},
			"S2001": {"Bommer Weiher", "Bommer Weiher", station.Lake},
			"F1042": {"Salmsacher Aach - Hefenhofen", "Salmsacher Aach", station.River},
			"00040": {"Rhein - Rheinfall oben", "Rhein", station.River},
			"00020": {"Fulach - Morgerenweiher", "Fulach", station.River},
			"00032": {"Durach - Merishausen", "Durach", station.River},
		},
	}
}

// NewStGallen reads the portal of the canton of St. Gallen (with some
// Appenzell stations): mostly smaller rivers and streams.
func NewStGallen(client *http.Client) *Wiski {
	return &Wiski{
		Client: client, LayerURL: "https://www.hydrodaten.sg.ch/wiskioutput/internet/layers/30/index.json",
		id: "sg", name: "Hydrodaten St. Gallen", site: "https://www.hydrodaten.sg.ch",
		bafuPrefix: "LH",
	}
}

func (w *Wiski) ID() string   { return w.id }
func (w *Wiski) Name() string { return w.name }
func (w *Wiski) URL() string  { return w.site }

func (w *Wiski) Fetch(ctx context.Context) ([]station.Station, error) {
	body, err := get(ctx, w.Client, w.LayerURL)
	if err != nil {
		return nil, err
	}
	return w.parse(body, time.Now())
}

// A station whose newest value is older than this is out of service.
const wiskiMaxAge = 48 * time.Hour

func (w *Wiski) parse(body []byte, now time.Time) ([]station.Station, error) {
	var rows []struct {
		No    string `json:"metadata_station_no"`
		Name  string `json:"metadata_station_name"`
		River string `json:"metadata_river_name"`
		// Thurgau sends numbers as strings, St. Gallen as numbers.
		Lat   flexFloat `json:"metadata_station_latitude"`
		Lon   flexFloat `json:"metadata_station_longitude"`
		Time  string    `json:"L1_timestamp"`
		Value flexFloat `json:"L1_ts_value"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	var out []station.Station
	for _, r := range rows {
		if r.No == "" || strings.HasPrefix(r.No, w.bafuPrefix) {
			continue
		}
		if r.Value.v == nil || *r.Value.v < -2 || *r.Value.v > 40 {
			continue
		}
		temp := *r.Value.v
		at, err := time.Parse(time.RFC3339, r.Time)
		if err != nil || now.Sub(at) > wiskiMaxAge {
			continue
		}
		s := station.Station{
			ID: w.id + "-" + strings.ToLower(r.No), Kind: station.River,
			Temperature: temp, MeasuredAt: &at, Source: w.id,
		}
		if r.Lat.v != nil && r.Lon.v != nil {
			s.Lat, s.Lon = r.Lat.v, r.Lon.v
		}
		if o, ok := w.stations[r.No]; ok {
			s.Name, s.WaterBody, s.Kind = o.name, o.water, o.kind
		} else {
			s.Name, s.WaterBody = wiskiName(r.Name, r.River, w.placeAfterSpace)
		}
		if s.Kind == station.Lake {
			s.HydroKey = lakeGauge(s.WaterBody)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errors.New("no current water temperatures")
	}
	return out, nil
}

// wiskiName turns the portals' station names into BAFU's "River - Place":
// "Thur_Niederbüren" and "Rhein Stein am Rhein" (river Rhein) become
// "Thur - Niederbüren" and "Rhein - Stein am Rhein"; "Simmi" stays as it is.
// With placeAfterSpace, "Lauche Matzingen" becomes "Lauche - Matzingen".
func wiskiName(name, river string, placeAfterSpace bool) (display, water string) {
	if river, place, ok := strings.Cut(name, "_"); ok {
		return river + " - " + place, river
	}
	if river != "" && strings.HasPrefix(name, river+" ") {
		return river + " - " + strings.TrimPrefix(name, river+" "), river
	}
	water, place, ok := strings.Cut(name, " ")
	if ok && placeAfterSpace {
		return water + " - " + place, water
	}
	// Some river names list several streams ("Feldbach/Speckbach").
	if river != "" && river != "---" && !strings.ContainsAny(river, "/(") {
		return name, river
	}
	return name, water
}
