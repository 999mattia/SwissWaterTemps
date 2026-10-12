package sources

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// WTLive reads wassertemperatur.live, battery-powered sensors that send the
// water temperature once an hour. Each sensor has its own page that shows only
// the latest value, without its time.
type WTLive struct {
	Client *http.Client
	// BaseURL is formatted with the sensor's subdomain.
	BaseURL string
}

func NewWTLive(client *http.Client) *WTLive {
	return &WTLive{Client: client, BaseURL: "https://%s.wassertemperatur.live/"}
}

func (w *WTLive) ID() string   { return "wtlive" }
func (w *WTLive) Name() string { return "wassertemperatur.live" }
func (w *WTLive) URL() string  { return "https://wassertemperatur.live" }

var wtliveSensors = []struct {
	sub, name, water string
	lat, lon         float64
}{
	{"biel-strandbad", "Bielersee – Strandbad Biel", "Bielersee", 47.1386, 7.2272},
	{"vingelz", "Bielersee – Vingelz", "Bielersee", 47.1332, 7.2186},
}

func (w *WTLive) Fetch(ctx context.Context) ([]station.Station, error) {
	var out []station.Station
	var errs []error
	for _, s := range wtliveSensors {
		body, err := get(ctx, w.Client, fmt.Sprintf(w.BaseURL, s.sub))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.sub, err))
			continue
		}
		temp, err := parseWTLive(body)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.sub, err))
			continue
		}
		out = append(out, station.Station{
			ID: "wtlive-" + s.sub, Name: s.name, WaterBody: s.water, Kind: station.Lake,
			Temperature: temp, Lat: ptr(s.lat), Lon: ptr(s.lon), Source: "wtlive", HydroKey: lakeGauge(s.water),
		})
	}
	return out, partialError(w.ID(), errs, len(wtliveSensors))
}

// parseWTLive reads "<h1>17.2&deg;C</h1>".
func parseWTLive(body []byte) (float64, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("parse HTML: %w", err)
	}
	temp, err := parseTemperature(doc.Find("h1").First().Text())
	if err != nil {
		return 0, err
	}
	if temp < -2 || temp > 40 {
		return 0, fmt.Errorf("implausible temperature %v", temp)
	}
	return temp, nil
}
