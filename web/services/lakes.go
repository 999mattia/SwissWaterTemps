package services

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/999mattia/SwissWaterTemps/models"
	"github.com/PuerkitoBio/goquery"
)

type WohlenseeTempData struct {
	Temperatur float64 `json:"temperatur"`
}

type SeeData struct {
	Wohlensee WohlenseeTempData `json:"wohlensee"`
}

type HikawetterApiResponse struct {
	See SeeData `json:"see"`
}

func GetLakeTemperatures(searchQuery ...string) []models.TemperatureRecord {
	url := "https://www.boot24.ch/chde/service/temperaturen/"

	var records []models.TemperatureRecord

	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find(".table__body .table__row").Each(func(i int, s *goquery.Selection) {
		name := s.Find(".table__cell .link").Text()
		temperatureStr := s.Find(".table__cell strong").Text()
		temperatureStr = strings.ReplaceAll(temperatureStr, "°", "")

		temperature, err := strconv.ParseFloat(temperatureStr, 64)
		if err != nil {
			log.Printf("error converting temperature: %s", err)
			return
		}

		record := models.TemperatureRecord{
			Name:        name,
			Temperature: temperature,
		}
		records = append(records, record)
	})

	records = append(records, GetWohlenseeTemperature())

	sort.Slice(records, func(i, j int) bool {
		return records[i].Name < records[j].Name
	})

	if len(searchQuery) > 0 && searchQuery[0] != "" {
		query := strings.ToLower(searchQuery[0])
		filteredRecords := []models.TemperatureRecord{}
		for _, record := range records {
			if strings.Contains(strings.ToLower(record.Name), query) {
				filteredRecords = append(filteredRecords, record)
			}
		}
		return filteredRecords
	}

	return records
}

func GetWohlenseeTemperature() models.TemperatureRecord {
	apiUrl := "https://hikawetter.ch/wetter/wetterdaten-json.php"
	wohlenseeRecord := models.TemperatureRecord{Name: "Wohlensee"}

	res, err := http.Get(apiUrl)
	if err != nil {
		log.Printf("Error fetching hikawetter API '%s': %v", apiUrl, err)
		return wohlenseeRecord
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Printf(
			"Error fetching hikawetter API '%s': status code %d %s",
			apiUrl,
			res.StatusCode,
			res.Status,
		)
		return wohlenseeRecord
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error reading hikawetter API response body: %v", err)
		return wohlenseeRecord
	}

	var apiData HikawetterApiResponse

	err = json.Unmarshal(bodyBytes, &apiData)
	if err != nil {
		log.Printf("Error parsing hikawetter API JSON: %v", err)
		return wohlenseeRecord
	}

	temperature := apiData.See.Wohlensee.Temperatur

	wohlenseeRecord.Temperature = temperature
	return wohlenseeRecord
}
