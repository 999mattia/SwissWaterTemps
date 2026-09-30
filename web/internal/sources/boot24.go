package sources

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Boot24 scrapes the lake temperature table on boot24.ch.
type Boot24 struct {
	Client  *http.Client
	PageURL string
}

func NewBoot24(client *http.Client) *Boot24 {
	return &Boot24{Client: client, PageURL: "https://www.boot24.ch/chde/service/temperaturen/"}
}

func (b *Boot24) ID() string   { return "boot24" }
func (b *Boot24) Name() string { return "boot24" }
func (b *Boot24) URL() string  { return b.PageURL }

func (b *Boot24) Fetch(ctx context.Context) ([]station.Station, error) {
	body, err := get(ctx, b.Client, b.PageURL)
	if err != nil {
		return nil, err
	}
	return parseBoot24(body)
}

var datePattern = regexp.MustCompile(`\b(\d{1,2})\.(\d{1,2})\.(\d{4})(?:,?\s+(\d{1,2}):(\d{2}))?`)

func parseBoot24(body []byte) ([]station.Station, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse boot24 HTML: %w", err)
	}

	var stations []station.Station
	seen := map[string]bool{}
	doc.Find(".table__body .table__row").Each(func(_ int, row *goquery.Selection) {
		name := strings.TrimSpace(row.Find(".table__cell .link").First().Text())
		temp, err := parseTemperature(row.Find(".table__cell strong").First().Text())
		if name == "" || err != nil {
			return
		}

		id := "boot24-" + slug(name)
		if seen[id] {
			return
		}
		seen[id] = true

		s := station.Station{
			ID:          id,
			Name:        name,
			WaterBody:   name,
			Kind:        station.Lake,
			Temperature: temp,
			MeasuredAt:  findDate(row.Text()),
			Source:      "boot24",
		}
		s.Lat, s.Lon = lakeCoordinates(name)
		s.HydroKey = lakeGauge(name)
		stations = append(stations, s)
	})

	// An empty result almost always means the page layout changed. Report it as
	// an error so the last good data is kept instead of silently showing nothing.
	if len(stations) == 0 {
		return nil, fmt.Errorf("boot24 page contains no temperature rows")
	}
	return stations, nil
}

// findDate picks up a "dd.mm.yyyy[ hh:mm]" timestamp if the row has one.
func findDate(s string) *time.Time {
	m := datePattern.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	layout, value := "2.1.2006", fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3])
	if m[4] != "" {
		layout, value = layout+" 15:04", fmt.Sprintf("%s %s:%s", value, m[4], m[5])
	}
	t, err := time.ParseInLocation(layout, value, zurich)
	if err != nil {
		return nil
	}
	return &t
}
