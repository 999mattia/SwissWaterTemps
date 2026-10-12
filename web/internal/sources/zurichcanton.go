package sources

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// ZurichCanton scrapes the current water temperatures of the canton of
// Zürich's hydrometry (AWEL), measured every 15 minutes: rivers and streams
// BAFU doesn't cover, and the Türlersee.
type ZurichCanton struct {
	Client  *http.Client
	PageURL string
}

func NewZurichCanton(client *http.Client) *ZurichCanton {
	return &ZurichCanton{Client: client, PageURL: "https://hydroproweb.zh.ch/Listen/AktuelleWerte/AktWassertemp.html"}
}

func (z *ZurichCanton) ID() string   { return "zh" }
func (z *ZurichCanton) Name() string { return "Kanton Zürich (AWEL)" }
func (z *ZurichCanton) URL() string {
	return "https://www.zh.ch/de/umwelt-tiere/wasser-gewaesser/messdaten/wassertemperaturen.html"
}

type zhStation struct {
	id, name, water string
	kind            station.Kind
	lat, lon        float64
}

// zhStations are the table's rows that are used, by their name in the table;
// positions from the canton's station map. Left out: the BAFU stations (marked
// "(BAFU)"), Zürichsee-Oberrieden, whose sensor reads far above the lake
// (24.5 °C in October), and Rhein - Ellikon a.R., which swings by 10 °C a day.
var zhStations = map[string]zhStation{
	"Aabach-Mönchaltorf":      {"527", "Aabach - Mönchaltorf", "Aabach", station.River, 47.3110, 8.7205},
	"Aabach-Niederuster":      {"554", "Aabach - Niederuster", "Aabach", station.River, 47.3465, 8.6972},
	"Eulach-Winterthur":       {"522", "Eulach - Winterthur", "Eulach", station.River, 47.4947, 8.7332},
	"Eulach-Wülflingen":       {"523", "Eulach - Wülflingen", "Eulach", station.River, 47.5094, 8.6880},
	"Furtbach-Würenlos":       {"548", "Furtbach - Würenlos", "Furtbach", station.River, 47.4446, 8.3655},
	"Geisslibach-Furtmüli":    {"576", "Geisslibach - Furtmüli", "Geisslibach", station.River, 47.6517, 8.7750},
	"Glatt-Dübendorf":         {"533", "Glatt - Dübendorf", "Glatt", station.River, 47.3973, 8.6229},
	"Glatt-Rümlang":           {"534", "Glatt - Rümlang", "Glatt", station.River, 47.4545, 8.5376},
	"Glatt-Wuhrbrücke":        {"531", "Glatt - Wuhrbrücke", "Glatt", station.River, 47.3731, 8.6551},
	"Jona-Rüti":               {"582", "Jona - Rüti", "Jona", station.River, 47.2488, 8.8528},
	"Jonen-Zwillikon":         {"574", "Jonen - Zwillikon", "Jonen", station.River, 47.2928, 8.4312},
	"Kempt-Illnau":            {"517", "Kempt - Illnau", "Kempt", station.River, 47.4157, 8.7202},
	"Kempt-Winterthur":        {"581", "Kempt - Winterthur", "Kempt", station.River, 47.4726, 8.7046},
	"Limmatkanal-KW Dietikon": {"586", "Limmat - Dietikon, Kraftwerkkanal", "Limmat", station.River, 47.4094, 8.4085},
	"Limmat-Zch. KW Letten":   {"578", "Limmat - Zürich, Kraftwerk Letten", "Limmat", station.River, 47.3878, 8.5314},
	"Reppisch-Dietikon":       {"572", "Reppisch - Dietikon", "Reppisch", station.River, 47.4111, 8.3984},
	"Rhein Rüdlingen":         {"597", "Rhein - Rüdlingen", "Rhein", station.River, 47.5755, 8.5779},
	"Sihl-Blattwag":           {"547", "Sihl - Blattwag", "Sihl", station.River, 47.1746, 8.6740},
	"Sihl-Zch.Sihlhölzli":     {"577", "Sihl - Zürich, Sihlhölzli", "Sihl", station.River, 47.3683, 8.5269},
	"Töss-Freienstein":        {"570", "Töss - Freienstein", "Töss", station.River, 47.5300, 8.5807},
	"Töss-Rämismühle":         {"520", "Töss - Rämismühle", "Töss", station.River, 47.4430, 8.8148},
	"Türlersee":               {"552", "Türlersee", "Türlersee", station.Lake, 47.2746, 8.4988},
}

func (z *ZurichCanton) Fetch(ctx context.Context) ([]station.Station, error) {
	body, err := get(ctx, z.Client, z.PageURL)
	if err != nil {
		return nil, err
	}
	return parseZurichCanton(body, time.Now())
}

func parseZurichCanton(body []byte, now time.Time) ([]station.Station, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var out []station.Station
	doc.Find("tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		// Name, unit, time + date, current, 24 h before, change, mean, max, min.
		if cells.Length() < 9 {
			return
		}
		cell := func(i int) string { return strings.TrimSpace(strings.ReplaceAll(cells.Eq(i).Text(), " ", " ")) }
		zs, ok := zhStations[cell(0)]
		if !ok {
			return
		}
		temp, err := parseTemperature(cell(3))
		if err != nil {
			return
		}
		// "10:45" and "12.10.2026" in one cell, local time.
		at, err := time.ParseInLocation("15:042.1.2006", strings.Join(strings.Fields(cell(2)), ""), zurich)
		if err != nil || now.Sub(at) > wiskiMaxAge {
			return
		}
		s := station.Station{
			ID: "zh-" + zs.id, Name: zs.name, WaterBody: zs.water, Kind: zs.kind,
			Temperature: temp, MeasuredAt: &at, Lat: ptr(zs.lat), Lon: ptr(zs.lon), Source: "zh",
		}
		if v, err := parseTemperature(cell(7)); err == nil {
			s.Max24h = &v
		}
		if v, err := parseTemperature(cell(8)); err == nil {
			s.Min24h = &v
		}
		out = append(out, s)
	})
	if len(out) == 0 {
		return nil, errors.New("page contains no known stations")
	}
	return out, nil
}
