package services

import "fmt"

func GetRiverTemperatures() {
	riverRecords := getRiverTemperatures()
	fmt.Println(riverRecords)
}

func GetLakeTemperatures() {
	lakeRecords := getLakeTemperatures()
	fmt.Println(lakeRecords)
}
