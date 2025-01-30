package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/999mattia/SwissWaterTemps/models"
)

func GetRiverTemperatures(searchQuery ...string) []models.TemperatureRecord {
	url := "https://www.hydrodaten.admin.ch/web-hydro-maps/hydro_sensor_temperature.geojson"

	resp, err := http.Get(url)
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Panic(err)
	}

	var data models.GeoJSONFile
	err = json.Unmarshal(body, &data)
	if err != nil {
		log.Panic(err)
	}

	records, err := castGeoJSONToTemperatureRecords(data)
	if err != nil {
		log.Panic(err)
	}

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

func castGeoJSONToTemperatureRecords(geoJSON models.GeoJSONFile) ([]models.TemperatureRecord, error) {
	var records []models.TemperatureRecord

	for _, feature := range geoJSON.Features {
		temp, err := strconv.ParseFloat(feature.Properties.LastValue, 64)
		if err != nil {
			fmt.Printf("Error parsing temperature for %s: %v\n", feature.Properties.Label, err)
			continue
		}

		record := models.TemperatureRecord{
			Name:        feature.Properties.Label,
			Temperature: temp,
		}

		records = append(records, record)
	}

	return records, nil
}
