package main

import (
	"github.com/999mattia/SwissWaterTemps/models"
	"github.com/999mattia/SwissWaterTemps/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
)

func main() {
	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{Views: engine})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{})
	})

	app.Get("/temperatures", func(c *fiber.Ctx) error {
		searchQuery := c.Query("search")

		var lakeTemperatures, riverTemperatures []models.TemperatureRecord

		if searchQuery != "" {
			lakeTemperatures = services.GetLakeTemperatures(searchQuery)
			riverTemperatures = services.GetRiverTemperatures(searchQuery)
		} else {
			lakeTemperatures = services.GetLakeTemperatures()
			riverTemperatures = services.GetRiverTemperatures()
		}

		return c.Render("temperatures", fiber.Map{"lakeTemperatures": lakeTemperatures, "riverTemperatures": riverTemperatures})
	})

	app.Get("/api/temperatures", func(c *fiber.Ctx) error {
		var lakeTemperatures, riverTemperatures []models.TemperatureRecord
		lakeTemperatures = services.GetLakeTemperatures()
		riverTemperatures = services.GetRiverTemperatures()

		return c.JSON(fiber.Map{
			"lakeTemperatures":  lakeTemperatures,
			"riverTemperatures": riverTemperatures,
		})
	})

	app.Static("/assets", "./assets")

	app.Listen(":3000")
}
