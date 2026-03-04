package api

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/marktsarkov/test/service"
)

func saveClick(service service.Iservice) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		bannerIDStr := c.Params("bannerID")
		bannerID, err := strconv.Atoi(bannerIDStr)
		if err != nil {
			return c.Status(400).SendString(fmt.Sprintf("invalid banner id: %d, bannerIDStr:%s", bannerID, bannerIDStr))
		}
		err = service.SaveClick(bannerID)
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		return c.SendString("ok")
	}
}

func getStats(service service.Iservice) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		bannerIDStr := c.Params("bannerID")
		bannerID, err := strconv.Atoi(bannerIDStr)
		if err != nil {
			return c.Status(400).SendString(fmt.Sprintf("invalid banner id: %s", bannerIDStr))
		}

		var requestBody struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := c.BodyParser(&requestBody); err != nil {
			return c.Status(400).SendString(fmt.Sprintf("invalid request body: %v", err))
		}
		tsFrom, err := time.Parse("2006-01-02T15:04:05", requestBody.From)
		if err != nil {
			return c.Status(400).SendString(fmt.Sprintf("invalid 'from' date format: %v", err))
		}
		tsTo, err := time.Parse("2006-01-02T15:04:05", requestBody.To)
		if err != nil {
			return c.Status(400).SendString(fmt.Sprintf("invalid 'to' date format: %v", err))
		}

		stats, err := service.GetStats(c.Context(), bannerID, tsFrom, tsTo)
		if err != nil {
			return c.Status(500).SendString(fmt.Sprintf("failed to get stats: %v", err))
		}

		type StatResponse struct {
			Ts string `json:"ts"`
			V  int    `json:"v"`
		}
		var response struct {
			Stats []StatResponse `json:"stats"`
		}
		for _, stat := range stats {
			response.Stats = append(response.Stats, StatResponse{
				Ts: stat.Ts.Format("2006-01-02T15:04:05"),
				V:  stat.Count,
			})
		}
		return c.JSON(response)
	}
}
