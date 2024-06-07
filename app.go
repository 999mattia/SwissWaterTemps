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

	app.Get("/main", func(c *fiber.Ctx) error {
		return c.Render("main", fiber.Map{})
	})

	app.Get("/rivers", func(c *fiber.Ctx) error {
		records := services.GetRiverTemperatures()
		return c.Render("rivers", fiber.Map{"records": records})
	})

	app.Get("/lakes", func(c *fiber.Ctx) error {
		records := services.GetLakeTemperatures()
		return c.Render("lakes", fiber.Map{"records": records})
	})

	app.Listen(":3000")
}
