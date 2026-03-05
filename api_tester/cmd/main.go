package main

import (
	"fmt"
	"log"

	"api_tester/config"
	"api_tester/internal/api"
	"api_tester/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфы: %v", err)
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.Default()

	// Инициал сервис и хендлер для маркировки
	markingService := service.NewMarkingService(cfg)
	markingHandler := api.NewMarkingHandler(markingService)

	v1 := r.Group("/v1")
	{
		markingGroup := v1.Group("/marking")
		{
			markingGroup.POST("/public-codes", markingHandler.GetPublicCodesInfo)
		}
		markingGroup.POST("/orders", markingHandler.CreateOrder)            // создание заказа
		markingGroup.GET("/orders", markingHandler.GetOrders)               // получение заказов
		markingGroup.GET("/codes", markingHandler.GetCodes)                 // получение кодов по заказу
		markingGroup.GET("/sub-orders", markingHandler.GetSubOrders)        // получение информации о выгрузках заказов
		markingGroup.POST("/utilisation", markingHandler.ReportUtilisation) // utilisation endpoint

	}

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "ping pong show:)",
		})
	})
	// server start
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Сервер запущен на порту %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}

}
