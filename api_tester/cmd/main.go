package main

import (
	"fmt"
	"log"
	"os"

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

	// ========== HEALTH CHECK (ДО СТАТИКИ) ==========
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "ASL Belgisi API Tester",
		})
	})

	// ========== ФРОНТЕНД ==========
	disableUI := os.Getenv("DISABLE_UI") == "true"
	if !disableUI {
		// Подаем статические файлы (CSS, JS)
		r.Static("/assets", "./web")
		// Подаем главную страницу
		r.StaticFile("/", "./web/index.html")
		// Fallback на index.html для SPA всех неизвестных путей
		r.NoRoute(func(c *gin.Context) {
			c.File("./web/index.html")
		})
		log.Println("✅ Веб-интерфейс доступен на http://localhost:8080")
	} else {
		log.Println("⚫ Веб-интерфейс отключен (DISABLE_UI=true)")
	}

	// Инициал сервис и хендлер для маркировки
	markingService := service.NewMarkingService(cfg)
	markingHandler := api.NewMarkingHandler(markingService)
	workflowService := service.NewWorkflowService(markingService, cfg)
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
			// Server-Sent Events для логов в реальном времени
			workflowGroup.GET("/logs", workflowHandler.GetLogs)
			// История логов
			workflowGroup.GET("/logs-history", workflowHandler.GetLogsHistory)
			// Экспортировать коды в CSV/TXT
			workflowGroup.POST("/export-codes", func(c *gin.Context) {
				workflowHandler.ExportCodes(c, []string{"code1", "code2", "code3"}) // Пример кодов
			})
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
		markingGroup.GET("/history", markingHandler.GetHistory)         // История операций для фронта
	}

	// Файлы кодов
	codesGroup := v1.Group("/codes")
	{
		codesGroup.GET("/files", api.ListCodeFiles)
		codesGroup.GET("/files/:filename", api.DownloadCodeFile)
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
