package main

import (
	"fmt"
	"log"

	"api_tester/config"
	"api_tester/internal/api"
	"api_tester/internal/db"
	"api_tester/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {

	db.InitDB()

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
	r.SetTrustedProxies(nil)

	// Инициал сервис и хендлер для маркировки
	markingService := service.NewMarkingService(cfg)
	markingHandler := api.NewMarkingHandler(markingService)
	workflowService := service.NewWorkflowService(markingService)
	workflowHandler := api.NewWorkflowHandler(workflowService)

	v1 := r.Group("/v1")
	{
		markingGroup := v1.Group("/marking")
		// cmd/main.go
		workflowGroup := v1.Group("/workflow")
		{
			// 🚀 ГЛАВНЫЙ ENDPOINT: Пользователь передает ТОЛЬКО gtin, productGroup, quantity
			workflowGroup.POST("/execute", workflowHandler.ExecuteWorkflow)
			// Полный workflow с полным OrderRequest (для продвинутых)
			workflowGroup.POST("/complete", workflowHandler.CompleteWorkflow)
			// Создание заказа и запуск полного цикла за один запрос
			workflowGroup.POST("/create-and-run", workflowHandler.CreateOrderAndRunFullCycle)
			// Запуск полного цикла для уже существующего заказа
			workflowGroup.POST("/run", workflowHandler.RunFullCycle)
			// Подача отчета об агрегации маркированных товаров
			workflowGroup.POST("/report-aggregation", workflowHandler.ReportAggregation)
		}
		{
			markingGroup.POST("/public-codes", markingHandler.GetPublicCodesInfo)
		}
		markingGroup.POST("/orders", markingHandler.CreateOrder)            // создание заказа
		markingGroup.GET("/orders", markingHandler.GetOrders)               // получение заказов
		markingGroup.GET("/codes", markingHandler.GetCodes)                 // получение кодов по заказу
		markingGroup.GET("/sub-orders", markingHandler.GetSubOrders)        // получение информации о выгрузках заказов
		markingGroup.POST("/utilisation", markingHandler.ReportUtilisation) // utilisation endpoint
		markingGroup.POST("/aggregation", markingHandler.ReportAggregation)
		markingGroup.GET("/generate-sscc", markingHandler.GenerateSSCC) // Генерация SSCC для тестов
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
	// Передаем storage в твой сервис или обработчик

}
