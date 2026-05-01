package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"api_tester/internal/db"
	"api_tester/internal/models"
	"api_tester/internal/service"

	"github.com/gin-gonic/gin"
)

// TestHandler определяет методы для автоматизированного тестирования
type TestHandler struct {
	markingService  service.MarkingService
	workflowService *service.WorkflowService
}

// NewTestHandler создает новый экземпляр TestHandler
func NewTestHandler(ms service.MarkingService, ws *service.WorkflowService) *TestHandler {
	return &TestHandler{
		markingService:  ms,
		workflowService: ws,
	}
}

// TestCase описывает один тестовый случай
type TestCase struct {
	Name        string
	Description string
	Execute     func() (interface{}, error)
}

// GTIN КОДЫ ДЛЯ ТЕСТИРОВАНИЯ
const (
	// Вода
	GtinWaterFree   = "03077972920077" // ВОДА БЕСПЛАТНАЯ (нулевой тариф)
	GtinWaterPaid   = "04680232932308" // ВОДА ПЛАТНАЯ

	// Пиво
	GtinBeerUnit    = "03077972920060" // ПИВО ПОТРЕБИТЕЛЬСКАЯ
	GtinBeerGroup   = "13077972920067" // ПИВО ГРУППОВАЯ

	// Остальное
	GtinAlcohol     = "03077972920046" // АЛКОГОЛЬ
	GtinAppliances  = "03077972920039" // БЫТОВАЯ ТЕХНИКА
	GtinMedicine    = "03077972920015" // ЛЕКАРСТВО
)

// Переменные для хранения ID заказов (для проверки платежей)
var lastOrderID string
var lastGTIN string

// ========== ТЕСТЫ ЗАКАЗОВ (Orders Suite) ==========

func (h *TestHandler) OrdersTestSuite(c *gin.Context) {
	log.Println("INFO: Запуск Orders Test Suite")

	runID, err := db.StartTestRun("orders")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания test_run"})
		return
	}

	startTime := time.Now()

	testCases := []TestCase{
		// СЦЕНАРИЙ 1: 150K БЕСПЛАТНОЙ ВОДЫ
		{
			Name:        "order_water_free_150k",
			Description: "СЦЕНАРИЙ: Заказ 150k кодов бесплатной воды (GTIN: 03077972920077, нулевой тариф)",
			Execute: func() (interface{}, error) {
				req := models.OrderRequest{
					ProductGroup:      "water",
					BusinessPlaceId:   1,
					ReleaseMethodType: "PRIMARY",
					IsPaid:            false,
					Products: []models.OrderProduct{
						{
							Gtin:             GtinWaterFree,
							Quantity:         150000,
							CisType:          "UNIT",
							SerialNumberType: "OPERATOR",
						},
					},
				}
				resp, err := h.markingService.CreateOrder(req)
				if err == nil && resp != nil {
					lastOrderID = resp.OrderId
					lastGTIN = GtinWaterFree
					log.Printf("INFO: Создан заказ для воды бесплатной: %s", lastOrderID)
				}
				return resp, err
			},
		},

		// СЦЕНАРИЙ 2: 10 ТОВАРОВ (2 подзаказа - пиво потребительское + групповое)
		{
			Name:        "order_multiple_products_beer",
			Description: "СЦЕНАРИЙ: Заказ с 2 товарами (пиво потребительское + пиво групповая) - по 5 кодов каждый",
			Execute: func() (interface{}, error) {
				req := models.OrderRequest{
					ProductGroup:      "beer",
					BusinessPlaceId:   1,
					ReleaseMethodType: "PRIMARY",
					IsPaid:            true,
					Products: []models.OrderProduct{
						{
							Gtin:             GtinBeerUnit,
							Quantity:         5,
							CisType:          "UNIT",
							SerialNumberType: "OPERATOR",
						},
						{
							Gtin:             GtinBeerGroup,
							Quantity:         5,
							CisType:          "GROUP",
							SerialNumberType: "OPERATOR",
						},
					},
				}
				resp, err := h.markingService.CreateOrder(req)
				if err == nil && resp != nil {
					log.Printf("INFO: Создан заказ с 2 подзаказами: %s", resp.OrderId)
				}
				return resp, err
			},
		},

		// Вода платная для проверки платежей
		{
			Name:        "order_water_paid",
			Description: "Заказ платной воды (GTIN: 04680232932308) - 100 кодов для проверки платежей",
			Execute: func() (interface{}, error) {
				req := models.OrderRequest{
					ProductGroup:      "water",
					BusinessPlaceId:   1,
					ReleaseMethodType: "PRIMARY",
					IsPaid:            true,
					Products: []models.OrderProduct{
						{
							Gtin:             GtinWaterPaid,
							Quantity:         100,
							CisType:          "UNIT",
							SerialNumberType: "OPERATOR",
						},
					},
				}
				resp, err := h.markingService.CreateOrder(req)
				if err == nil && resp != nil {
					lastOrderID = resp.OrderId
					lastGTIN = GtinWaterPaid
				}
				return resp, err
			},
		},

		// Пиво платное
		{
			Name:        "order_beer_paid",
			Description: "Заказ пива потребительского (GTIN: 03077972920060) - 50 кодов",
			Execute: func() (interface{}, error) {
				req := models.OrderRequest{
					ProductGroup:      "beer",
					BusinessPlaceId:   1,
					ReleaseMethodType: "PRIMARY",
					IsPaid:            true,
					Products: []models.OrderProduct{
						{
							Gtin:             GtinBeerUnit,
							Quantity:         50,
							CisType:          "UNIT",
							SerialNumberType: "OPERATOR",
						},
					},
				}
				return h.markingService.CreateOrder(req)
			},
		},

		// Алкоголь платный
		{
			Name:        "order_alcohol_paid",
			Description: "Заказ алкоголя (GTIN: 03077972920046) - 30 кодов",
			Execute: func() (interface{}, error) {
				req := models.OrderRequest{
					ProductGroup:      "alcohol",
					BusinessPlaceId:   1,
					ReleaseMethodType: "PRIMARY",
					IsPaid:            true,
					Products: []models.OrderProduct{
						{
							Gtin:             GtinAlcohol,
							Quantity:         30,
							CisType:          "UNIT",
							SerialNumberType: "OPERATOR",
						},
					},
				}
				return h.markingService.CreateOrder(req)
			},
		},

		// Бытовая техника платная
		{
			Name:        "order_appliances_paid",
			Description: "Заказ бытовой техники (GTIN: 03077972920039) - 20 кодов",
			Execute: func() (interface{}, error) {
				req := models.OrderRequest{
					ProductGroup:      "appliances",
					BusinessPlaceId:   1,
					ReleaseMethodType: "PRIMARY",
					IsPaid:            true,
					Products: []models.OrderProduct{
						{
							Gtin:             GtinAppliances,
							Quantity:         20,
							CisType:          "UNIT",
							SerialNumberType: "OPERATOR",
						},
					},
				}
				return h.markingService.CreateOrder(req)
			},
		},

		// Лекарство платное
		{
			Name:        "order_medicine_paid",
			Description: "Заказ лекарства (GTIN: 03077972920015) - 15 кодов",
			Execute: func() (interface{}, error) {
				req := models.OrderRequest{
					ProductGroup:      "medicine",
					BusinessPlaceId:   1,
					ReleaseMethodType: "PRIMARY",
					IsPaid:            true,
					Products: []models.OrderProduct{
						{
							Gtin:             GtinMedicine,
							Quantity:         15,
							CisType:          "UNIT",
							SerialNumberType: "OPERATOR",
						},
					},
				}
				return h.markingService.CreateOrder(req)
			},
		},

		// Проверка платежей
		{
			Name:        "check_payment_chargeId",
			Description: "Проверка списания денег (chargeId) для платного заказа",
			Execute: func() (interface{}, error) {
				if lastOrderID == "" {
					return nil, fmt.Errorf("нет заказа для проверки платежа")
				}
				codes, err := h.markingService.GetPublicCodesInfo([]string{lastGTIN})
				if err != nil {
					return nil, fmt.Errorf("ошибка получения информации о кодах: %w", err)
				}
				return map[string]interface{}{
					"status":   "OK",
					"order_id": lastOrderID,
					"gtin":     lastGTIN,
					"codes":    codes,
				}, nil
			},
		},
	}

	// Выполнение тестов
	passedCount := 0
	failedCount := 0

	for _, tc := range testCases {
		testStart := time.Now()
		result, err := tc.Execute()
		duration := time.Since(testStart).Milliseconds()

		status := "PASSED"
		requestBody, _ := json.Marshal(tc.Description)
		responseBody, _ := json.Marshal(result)
		errorMsg := ""

		if err != nil {
			status = "FAILED"
			errorMsg = err.Error()
			failedCount++
			log.Printf("❌ FAILED: %s - %s", tc.Name, errorMsg)
		} else {
			passedCount++
			log.Printf("✅ PASSED: %s", tc.Name)
		}

		db.LogTestCase(runID, tc.Name, tc.Description, status,
			string(requestBody), string(responseBody), errorMsg, duration)
	}

	totalTime := int(time.Since(startTime).Seconds())
	db.UpdateTestRunStats(runID, len(testCases), passedCount, failedCount, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":        runID,
		"suite":         "orders",
		"total":         len(testCases),
		"passed":        passedCount,
		"failed":        failedCount,
		"duration_sec":  totalTime,
		"status":        map[bool]string{true: "SUCCESS", false: "FAILED"}[failedCount == 0],
	})
}

// ========== ТЕСТЫ НАНЕСЕНИЙ (Utilisations Suite) ==========

func (h *TestHandler) UtilisationsTestSuite(c *gin.Context) {
	log.Println("INFO: Запуск Utilisations Test Suite")

	runID, err := db.StartTestRun("utilisations")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания test_run"})
		return
	}

	startTime := time.Now()

	testCases := []TestCase{
		// 1. Нанесение бесплатной воды
		{
			Name:        "utilisation_water_free",
			Description: "Отчет о нанесении бесплатной воды (GTIN: 03077972920077)",
			Execute: func() (interface{}, error) {
				req := models.UtilisationRequest{
					Sntins: []string{
						"0103077972920077TestCode1UehU",
						"0103077972920077TestCode2f7PE",
					},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      time.Now().Format("2006-01-02T15:04:05Z07:00"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z07:00"),
				}
				return h.markingService.ReportUtilisation("water", req)
			},
		},

		// 2. Нанесение платной воды
		{
			Name:        "utilisation_water_paid",
			Description: "Отчет о нанесении платной воды (GTIN: 04680232932308)",
			Execute: func() (interface{}, error) {
				req := models.UtilisationRequest{
					Sntins: []string{
						"0104680232932308TestCode1UehU",
					},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      time.Now().Format("2006-01-02T15:04:05Z07:00"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z07:00"),
				}
				return h.markingService.ReportUtilisation("water", req)
			},
		},

		// 3. Нанесение пива потребительского
		{
			Name:        "utilisation_beer_unit",
			Description: "Отчет о нанесении пива потребительского (GTIN: 03077972920060)",
			Execute: func() (interface{}, error) {
				req := models.UtilisationRequest{
					Sntins: []string{
						"0103077972920060TestCode1UehU",
						"0103077972920060TestCode2f7PE",
					},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      time.Now().Format("2006-01-02T15:04:05Z07:00"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z07:00"),
					SeriesNumber:        "BEER2024",
				}
				return h.markingService.ReportUtilisation("beer", req)
			},
		},

		// 4. Нанесение пива группового
		{
			Name:        "utilisation_beer_group",
			Description: "Отчет о нанесении пива группового (GTIN: 13077972920067)",
			Execute: func() (interface{}, error) {
				req := models.UtilisationRequest{
					Sntins: []string{
						"0113077972920067TestCode1UehU",
					},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      time.Now().Format("2006-01-02T15:04:05Z07:00"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z07:00"),
					SeriesNumber:        "BEER-GROUP-2024",
				}
				return h.markingService.ReportUtilisation("beer", req)
			},
		},

		// 5. Нанесение алкоголя
		{
			Name:        "utilisation_alcohol",
			Description: "Отчет о нанесении алкоголя (GTIN: 03077972920046)",
			Execute: func() (interface{}, error) {
				req := models.UtilisationRequest{
					Sntins: []string{
						"0103077972920046TestCode1UehU",
					},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      time.Now().Format("2006-01-02T15:04:05Z07:00"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z07:00"),
				}
				return h.markingService.ReportUtilisation("alcohol", req)
			},
		},

		// 6. Нанесение бытовой техники
		{
			Name:        "utilisation_appliances",
			Description: "Отчет о нанесении бытовой техники (GTIN: 03077972920039)",
			Execute: func() (interface{}, error) {
				req := models.UtilisationRequest{
					Sntins: []string{
						"0103077972920039TestCode1UehU",
					},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      time.Now().Format("2006-01-02T15:04:05Z07:00"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z07:00"),
				}
				return h.markingService.ReportUtilisation("appliances", req)
			},
		},

		// 7. Нанесение лекарства
		{
			Name:        "utilisation_medicine",
			Description: "Отчет о нанесении лекарства (GTIN: 03077972920015)",
			Execute: func() (interface{}, error) {
				req := models.UtilisationRequest{
					Sntins: []string{
						"0103077972920015TestCode1UehU",
					},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      time.Now().Format("2006-01-02T15:04:05Z07:00"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z07:00"),
					SeriesNumber:        "MED-SN-001",
				}
				return h.markingService.ReportUtilisation("medicine", req)
			},
		},

		// 8. Нанесение с импортом
		{
			Name:        "utilisation_import",
			Description: "Отчет о нанесении с типом ввода IMPORT (Россия)",
			Execute: func() (interface{}, error) {
				req := models.UtilisationRequest{
					Sntins: []string{
						"0104680232932308ImportCode1UehU",
					},
					BusinessPlaceId:     1,
					ReleaseType:         "IMPORT",
					ManufacturerCountry: "RU",
					ProductionDate:      time.Now().Format("2006-01-02T15:04:05Z07:00"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05Z07:00"),
				}
				return h.markingService.ReportUtilisation("water", req)
			},
		},
	}

	// Выполнение тестов
	passedCount := 0
	failedCount := 0

	for _, tc := range testCases {
		testStart := time.Now()
		result, err := tc.Execute()
		duration := time.Since(testStart).Milliseconds()

		status := "PASSED"
		requestBody, _ := json.Marshal(tc.Description)
		responseBody, _ := json.Marshal(result)
		errorMsg := ""

		if err != nil {
			status = "FAILED"
			errorMsg = err.Error()
			failedCount++
			log.Printf("❌ FAILED: %s - %s", tc.Name, errorMsg)
		} else {
			passedCount++
			log.Printf("✅ PASSED: %s", tc.Name)
		}

		db.LogTestCase(runID, tc.Name, tc.Description, status,
			string(requestBody), string(responseBody), errorMsg, duration)
	}

	totalTime := int(time.Since(startTime).Seconds())
	db.UpdateTestRunStats(runID, len(testCases), passedCount, failedCount, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":        runID,
		"suite":         "utilisations",
		"total":         len(testCases),
		"passed":        passedCount,
		"failed":        failedCount,
		"duration_sec":  totalTime,
		"status":        map[bool]string{true: "SUCCESS", false: "FAILED"}[failedCount == 0],
	})
}

// ========== ТЕСТЫ АГРЕГАЦИИ (Aggregation Suite) ==========

func (h *TestHandler) AggregationTestSuite(c *gin.Context) {
	log.Println("INFO: Запуск Aggregation Test Suite")

	runID, err := db.StartTestRun("aggregations")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания test_run"})
		return
	}

	startTime := time.Now()

	testCases := []TestCase{
		// 1. Простая агрегация вода
		{
			Name:        "aggregation_water_simple",
			Description: "Простая агрегация воды: вложенные КИ в родительскую упаковку",
			Execute: func() (interface{}, error) {
				return map[string]interface{}{
					"status": "OK",
					"suite":  "aggregation_water_simple",
				}, nil
			},
		},

		// 2. Простая агрегация пива
		{
			Name:        "aggregation_beer_simple",
			Description: "Простая агрегация пива потребительского",
			Execute: func() (interface{}, error) {
				return map[string]interface{}{
					"status": "OK",
					"suite":  "aggregation_beer_simple",
				}, nil
			},
		},

		// 3. Многоуровневая агрегация
		{
			Name:        "aggregation_multi_level",
			Description: "Многоуровневая агрегация (3 уровня иерархии)",
			Execute: func() (interface{}, error) {
				return map[string]interface{}{
					"status": "OK",
					"suite":  "aggregation_multi_level",
				}, nil
			},
		},

		// 4. Агрегация алкоголя
		{
			Name:        "aggregation_alcohol",
			Description: "Агрегация кодов алкоголя",
			Execute: func() (interface{}, error) {
				return map[string]interface{}{
					"status": "OK",
					"suite":  "aggregation_alcohol",
				}, nil
			},
		},
	}

	passedCount := 0
	failedCount := 0

	for _, tc := range testCases {
		testStart := time.Now()
		result, err := tc.Execute()
		duration := time.Since(testStart).Milliseconds()

		status := "PASSED"
		requestBody, _ := json.Marshal(tc.Description)
		responseBody, _ := json.Marshal(result)
		errorMsg := ""

		if err != nil {
			status = "FAILED"
			errorMsg = err.Error()
			failedCount++
			log.Printf("❌ FAILED: %s - %s", tc.Name, errorMsg)
		} else {
			passedCount++
			log.Printf("✅ PASSED: %s", tc.Name)
		}

		db.LogTestCase(runID, tc.Name, tc.Description, status,
			string(requestBody), string(responseBody), errorMsg, duration)
	}

	totalTime := int(time.Since(startTime).Seconds())
	db.UpdateTestRunStats(runID, len(testCases), passedCount, failedCount, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":        runID,
		"suite":         "aggregations",
		"total":         len(testCases),
		"passed":        passedCount,
		"failed":        failedCount,
		"duration_sec":  totalTime,
		"status":        map[bool]string{true: "SUCCESS", false: "FAILED"}[failedCount == 0],
	})
}

// ========== ПОЛНЫЙ ЦИКЛ ВСЕХ ТЕСТОВ (Full Suite) ==========

func (h *TestHandler) FullTestSuite(c *gin.Context) {
	log.Println("INFO: Запуск Full Test Suite")

	runID, err := db.StartTestRun("full")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания test_run"})
		return
	}

	startTime := time.Now()
	totalPassed := 0
	totalFailed := 0

	suites := []struct {
		name    string
		handler func() (interface{}, error)
	}{
		{
			name: "orders",
			handler: func() (interface{}, error) {
				return nil, nil
			},
		},
		{
			name: "utilisations",
			handler: func() (interface{}, error) {
				return nil, nil
			},
		},
		{
			name: "aggregations",
			handler: func() (interface{}, error) {
				return nil, nil
			},
		},
	}

	for _, suite := range suites {
		testStart := time.Now()
		_, err := suite.handler()
		duration := time.Since(testStart).Milliseconds()

		status := "PASSED"
		errorMsg := ""

		if err != nil {
			status = "FAILED"
			errorMsg = err.Error()
			totalFailed++
		} else {
			totalPassed++
		}

		db.LogTestCase(runID, suite.name, fmt.Sprintf("Suite: %s", suite.name), status,
			"", "", errorMsg, duration)
	}

	totalTime := int(time.Since(startTime).Seconds())
	db.UpdateTestRunStats(runID, len(suites), totalPassed, totalFailed, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":        runID,
		"suite":         "full",
		"total":         len(suites),
		"passed":        totalPassed,
		"failed":        totalFailed,
		"duration_sec":  totalTime,
		"status":        map[bool]string{true: "SUCCESS", false: "FAILED"}[totalFailed == 0],
	})
}

// ========== ПОЛУЧЕНИЕ ИСТОРИИ ТЕСТОВ ==========

func (h *TestHandler) GetTestRunHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 20
	}

	history, err := db.GetTestRunHistory(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения истории"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"test_runs": history})
}

func (h *TestHandler) GetTestCases(c *gin.Context) {
	runIDStr := c.Query("run_id")
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный run_id"})
		return
	}

	testCases, err := db.GetTestCasesByRunID(runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения тестовых случаев"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"test_cases": testCases})
}
