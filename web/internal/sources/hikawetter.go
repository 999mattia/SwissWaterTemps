package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// HikaWetter reads the Wohlensee temperature from the HiKa weather station.
type HikaWetter struct {
	Client  *http.Client
	DataURL string
}

func NewHikaWetter(client *http.Client) *HikaWetter {
	return &HikaWetter{Client: client, DataURL: "https://hikawetter.ch/wetter/wetterdaten-json.php"}
}

func (h *HikaWetter) ID() string   { return "hikawetter" }
func (h *HikaWetter) Name() string { return "HiKa Wetter" }
func (h *HikaWetter) URL() string  { return "https://hikawetter.ch" }

func (h *HikaWetter) Fetch(ctx context.Context) ([]station.Station, error) {
	body, err := get(ctx, h.Client, h.DataURL)
	if err != nil {
		return nil, err
	}
	return parseHikaWetter(body)
}

func parseHikaWetter(body []byte) ([]station.Station, error) {
	var data struct {
		See struct {
			Wohlensee struct {
				Temperatur flexFloat `json:"temperatur"`
			} `json:"wohlensee"`
		} `json:"see"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decode HiKa Wetter JSON: %w", err)
	}

	temp := data.See.Wohlensee.Temperatur.v
	if temp == nil {
		return nil, fmt.Errorf("HiKa Wetter JSON has no Wohlensee temperature")
	}

	s := station.Station{
		ID:          "hikawetter-wohlensee",
		Name:        "Wohlensee",
		WaterBody:   "Wohlensee",
		Kind:        station.Lake,
		Temperature: *temp,
		Source:      "hikawetter",
	}
	s.Lat, s.Lon = lakeCoordinates(s.Name)
	return []station.Station{s}, nil
}
