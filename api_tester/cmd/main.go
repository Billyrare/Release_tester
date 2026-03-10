package main

import (
	"fmt"
	"log"
	"os"

	"api_tester/config"
	"api_tester/internal/api"
	"api_tester/internal/db"
	_ "api_tester/internal/metrics" // Регистрация Prometheus-метрик при старте
	"api_tester/internal/middleware"
	"api_tester/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	// Prometheus HTTP middleware
	r.Use(middleware.PrometheusMiddleware())

	// ========== PROMETHEUS METRICS ==========
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	log.Println("📊 Prometheus метрики доступны на http://localhost:8080/metrics")

	// ========== HEALTH CHECK ==========
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "ASL Belgisi API Tester",
		})
	})

	// ========== ФРОНТЕНД ==========
	disableUI := os.Getenv("DISABLE_UI") == "true"
	if !disableUI {
		r.Static("/assets", "./web")
		r.StaticFile("/", "./web/index.html")
		r.NoRoute(func(c *gin.Context) {
			c.File("./web/index.html")
		})
		log.Println("✅ Веб-интерфейс доступен на http://localhost:8080")
	} else {
		log.Println("⚫ Веб-интерфейс отключен (DISABLE_UI=true)")
	}

	// Инициализация сервисов
	markingService := service.NewMarkingService(cfg)
	markingHandler := api.NewMarkingHandler(markingService)
	workflowService := service.NewWorkflowService(markingService, cfg)
	workflowHandler := api.NewWorkflowHandler(workflowService)

	v1 := r.Group("/v1")
	{
		markingGroup := v1.Group("/marking")
		workflowGroup := v1.Group("/workflow")
		{
			workflowGroup.POST("/execute", workflowHandler.ExecuteWorkflow)
			workflowGroup.POST("/complete", workflowHandler.CompleteWorkflow)
			workflowGroup.POST("/create-and-run", workflowHandler.CreateOrderAndRunFullCycle)
			workflowGroup.POST("/run", workflowHandler.RunFullCycle)
			workflowGroup.POST("/report-aggregation", workflowHandler.ReportAggregation)
			workflowGroup.GET("/logs", workflowHandler.GetLogs)
			workflowGroup.GET("/logs-history", workflowHandler.GetLogsHistory)
			workflowGroup.POST("/export-codes", func(c *gin.Context) {
				workflowHandler.ExportCodes(c, []string{"code1", "code2", "code3"})
			})
		}
		{
			markingGroup.POST("/public-codes", markingHandler.GetPublicCodesInfo)
		}
		markingGroup.POST("/orders", markingHandler.CreateOrder)
		markingGroup.GET("/orders", markingHandler.GetOrders)
		markingGroup.GET("/codes", markingHandler.GetCodes)
		markingGroup.GET("/sub-orders", markingHandler.GetSubOrders)
		markingGroup.POST("/utilisation", markingHandler.ReportUtilisation)
		markingGroup.POST("/aggregation", markingHandler.ReportAggregation)
		markingGroup.GET("/generate-sscc", markingHandler.GenerateSSCC)
		markingGroup.GET("/history", markingHandler.GetHistory)
	}

	// Файлы кодов
	codesGroup := v1.Group("/codes")
	{
		codesGroup.GET("/files", api.ListCodeFiles)
		codesGroup.GET("/files/:filename", api.DownloadCodeFile)
	}

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "ping pong show:)"})
	})

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Сервер запущен на порту %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
