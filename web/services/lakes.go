package services

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/999mattia/SwissWaterTemps/models"
	"github.com/PuerkitoBio/goquery"
)

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
