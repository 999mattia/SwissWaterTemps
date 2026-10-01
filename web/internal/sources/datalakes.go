package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Datalakes reads the surface temperature of Eawag's monitoring buoys and
// platforms from the Datalakes API. Every dataset stores one JSON file per
// day; the file list is long (hundreds of KB), so the current day's file id is
// cached and the list only re-read when the dataset reports a newer day.
type Datalakes struct {
	Client  *http.Client
	BaseURL string

	mu    sync.Mutex
	cache map[int]*datalakesCache
}

type datalakesCache struct {
	axes    map[string]string // parameter name → axis key in the data file ("y", "y1" …)
	fileID  int
	fileDay string // UTC date of the cached file's newest value
	last    *station.Station
}

func NewDatalakes(client *http.Client) *Datalakes {
	return &Datalakes{Client: client, BaseURL: "https://api.datalakes-eawag.ch", cache: map[int]*datalakesCache{}}
}

func (d *Datalakes) ID() string   { return "datalakes" }
func (d *Datalakes) Name() string { return "Eawag Datalakes" }
func (d *Datalakes) URL() string  { return "https://www.datalakes-eawag.ch" }

type datalakesSet struct {
	id          int
	param, qual string // surface temperature and its quality flag (0 = ok), if any
	name, water string
	lat, lon    float64
}

// Datasets with a live surface temperature. Positions are the buoys'.
var datalakesSets = []datalakesSet{
	{1264, "temperature", "temperature_qual", "Vierwaldstättersee – Kastanienbaum", "Vierwaldstättersee", 47.000, 8.330},
	{448, "surfacetemp", "", "Genfersee – Pully (LéXPLORE)", "Genfersee", 46.500, 6.670},
	{956, "surfacetemp", "", "Murtensee – Seemitte", "Murtensee", 46.934, 7.077},
	{1046, "surfacetemp", "", "Hallwilersee – Seemitte", "Hallwilersee", 47.2785, 8.2138},
	{1074, "surface_temp", "surface_temp_qual", "Ägerisee – Seemitte", "Ägerisee", 47.1249, 8.6085},
	{1334, "surface_temp", "", "Lago di Lugano – Castagnola", "Lago di Lugano", 45.9993, 8.9803},
	{1443, "surfacetemp", "", "Lago di Lugano – Figino", "Lago di Lugano", 45.9594, 8.8929},
	{1333, "surface_temp", "", "Lago Maggiore – Porto Ronco", "Lago Maggiore", 46.144, 8.731},
}

// A dataset whose newest value is older than this is left out.
const datalakesMaxAge = 48 * time.Hour

// Fetch reports an error only when no dataset could be read: single buoys are
// regularly out of service (maintenance, winter), which must not mark the whole
// source as unreachable. Their failures are logged instead.
func (d *Datalakes) Fetch(ctx context.Context) ([]station.Station, error) {
	var out []station.Station
	var errs []error
	for _, set := range datalakesSets {
		s, err := d.fetchSet(ctx, set)
		d.mu.Lock()
		c := d.cache[set.id]
		if err != nil {
			err = fmt.Errorf("dataset %d: %w", set.id, err)
			slog.Warn("fetching Datalakes dataset failed", "error", err)
			errs = append(errs, err)
		} else {
			c.last = s
		}
		// A failed dataset keeps its last value instead of vanishing.
		if c != nil && c.last != nil && time.Since(*c.last.MeasuredAt) < datalakesMaxAge {
			out = append(out, *c.last)
		}
		d.mu.Unlock()
	}
	if len(errs) == len(datalakesSets) {
		return out, errors.Join(errs...)
	}
	return out, nil
}

func (d *Datalakes) fetchSet(ctx context.Context, set datalakesSet) (*station.Station, error) {
	d.mu.Lock()
	c := d.cache[set.id]
	if c == nil {
		c = &datalakesCache{}
		d.cache[set.id] = c
	}
	d.mu.Unlock()

	if c.axes == nil {
		body, err := get(ctx, d.Client, fmt.Sprintf("%s/datasetparameters?datasets_id=%d", d.BaseURL, set.id))
		if err != nil {
			return nil, err
		}
		var params []struct {
			Axis  string `json:"axis"`
			Parse string `json:"parseparameter"`
		}
		if err := json.Unmarshal(body, &params); err != nil {
			return nil, fmt.Errorf("decode parameters: %w", err)
		}
		axes := map[string]string{}
		for _, p := range params {
			axes[p.Parse] = p.Axis
		}
		if axes["time"] == "" || axes[set.param] == "" {
			return nil, fmt.Errorf("parameters time/%s not found", set.param)
		}
		c.axes = axes
	}

	body, err := get(ctx, d.Client, fmt.Sprintf("%s/datasets/%d", d.BaseURL, set.id))
	if err != nil {
		return nil, err
	}
	var meta struct {
		MaxDatetime string `json:"maxdatetime"`
	}
	if err := json.Unmarshal(body, &meta); err != nil || len(meta.MaxDatetime) < 10 {
		return nil, fmt.Errorf("decode dataset: %v", err)
	}
	if c.fileID == 0 || meta.MaxDatetime[:10] != c.fileDay {
		if err := d.findNewestFile(ctx, set.id, c); err != nil {
			return nil, err
		}
	}

	body, err = get(ctx, d.Client, fmt.Sprintf("%s/files/%d?get=raw", d.BaseURL, c.fileID))
	if err != nil {
		return nil, err
	}
	points, err := parseDatalakes(body, c.axes["time"], c.axes[set.param], c.axes[set.qual])
	if err != nil {
		return nil, err
	}
	last := points[len(points)-1]
	return &station.Station{
		ID: "datalakes-" + strconv.Itoa(set.id), Name: set.name, WaterBody: set.water, Kind: station.Lake,
		Temperature: math.Round(last.Value*10) / 10, MeasuredAt: &last.Time, Lat: ptr(set.lat), Lon: ptr(set.lon),
		Source: "datalakes", HydroKey: lakeGauge(set.water), Recent: hourly(points),
	}, nil
}

func (d *Datalakes) findNewestFile(ctx context.Context, id int, c *datalakesCache) error {
	body, err := get(ctx, d.Client, fmt.Sprintf("%s/files?datasets_id=%d", d.BaseURL, id))
	if err != nil {
		return err
	}
	var files []struct {
		ID          int    `json:"id"`
		Type        string `json:"filetype"`
		MaxDatetime string `json:"maxdatetime"`
	}
	if err := json.Unmarshal(body, &files); err != nil {
		return fmt.Errorf("decode files: %w", err)
	}
	newest := -1
	for i, f := range files {
		if f.Type == "json" && len(f.MaxDatetime) >= 10 && (newest < 0 || f.MaxDatetime > files[newest].MaxDatetime) {
			newest = i
		}
	}
	if newest < 0 {
		return errors.New("no JSON data file")
	}
	c.fileID, c.fileDay = files[newest].ID, files[newest].MaxDatetime[:10]
	return nil
}

// parseDatalakes returns the valid points of one daily file, oldest first:
// value present, not NaN, and not flagged by the quality axis (if there is one
// and it covers the series).
func parseDatalakes(body []byte, timeAxis, valueAxis, qualAxis string) ([]station.Point, error) {
	var file map[string]json.RawMessage
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("decode data: %w", err)
	}
	var times []*float64
	var values, quals []*float64
	if err := json.Unmarshal(file[timeAxis], &times); err != nil {
		return nil, fmt.Errorf("time axis: %w", err)
	}
	if err := json.Unmarshal(file[valueAxis], &values); err != nil || len(values) != len(times) {
		return nil, fmt.Errorf("value axis %s: %v", valueAxis, err)
	}
	if qualAxis != "" {
		if json.Unmarshal(file[qualAxis], &quals) != nil || len(quals) != len(times) {
			quals = nil // flags missing for this file: use the values as they are
		}
	}

	var points []station.Point
	for i, t := range times {
		v := values[i]
		if t == nil || v == nil || math.IsNaN(*v) || *v < -2 || *v > 40 {
			continue
		}
		if quals != nil && quals[i] != nil && *quals[i] != 0 {
			continue
		}
		points = append(points, station.Point{Time: time.Unix(int64(*t), 0).UTC(), Value: *v})
	}
	if len(points) == 0 {
		return nil, errors.New("no valid values")
	}
	return points, nil
}

// hourly keeps the last point of every hour; history is stored hourly anyway.
func hourly(points []station.Point) []station.Point {
	var out []station.Point
	for i, p := range points {
		if i == len(points)-1 || !p.Time.Truncate(time.Hour).Equal(points[i+1].Time.Truncate(time.Hour)) {
			out = append(out, p)
		}
	}
	return out
}
