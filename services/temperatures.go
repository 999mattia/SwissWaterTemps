package services

import "fmt"

func GetAllTemperatures() {
	riverRecords := getRiverTemperatures()
	fmt.Println(riverRecords)
}
