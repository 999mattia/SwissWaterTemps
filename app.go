package main

import (
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
		lakeTemperatures := services.GetLakeTemperatures()
		riverTemperatures := services.GetRiverTemperatures()
		return c.Render("temperatures", fiber.Map{"lakeTemperatures": lakeTemperatures, "riverTemperatures": riverTemperatures})
	})

	app.Listen(":3000")
}
