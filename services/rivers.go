package services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/999mattia/SwissWaterTemps/models"
)

func getRiverTemperatures() []models.TemperatureRecord {
	url := "https://www.hydrodaten.admin.ch/web-hydro-maps/hydro_sensor_temperature.geojson"

	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var data models.GeoJSONFile
	err = json.Unmarshal(body, &data)
	if err != nil {
		panic(err)
	}

	records, err := CastGeoJSONToTemperatureRecords(data)
	if err != nil {
		panic(err)
	}

	return records
}

func CastGeoJSONToTemperatureRecords(geoJSON models.GeoJSONFile) ([]models.TemperatureRecord, error) {
	var records []models.TemperatureRecord

	for _, feature := range geoJSON.Features {
		// Parse the LastValue to float64
		temp, err := strconv.ParseFloat(feature.Properties.LastValue, 64)
		if err != nil {
			// Handle the error, maybe continue with the next feature or return the error
			fmt.Printf("Error parsing temperature for %s: %v\n", feature.Properties.Label, err)
			continue // or return nil, err
		}

		record := models.TemperatureRecord{
			Name:        feature.Properties.Label,
			Temperature: temp,
		}

		records = append(records, record)
	}

	return records, nil
}
