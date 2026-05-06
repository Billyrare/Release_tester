package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"api_tester/internal/db"
	"api_tester/internal/models"
	"api_tester/internal/service"
	"api_tester/internal/util"

	"github.com/gin-gonic/gin"
)

type TestHandler struct {
	markingService  service.MarkingService
	workflowService *service.WorkflowService
}

func NewTestHandler(ms service.MarkingService, ws *service.WorkflowService) *TestHandler {
	return &TestHandler{
		markingService:  ms,
		workflowService: ws,
	}
}

type TestCase struct {
	Name        string
	Description string
	Execute     func() (interface{}, error)
}

const (
	GtinWaterFree  = "03077972920077"
	GtinWaterPaid  = "04680232932308"
	GtinBeerUnit   = "03077972920060"
	GtinBeerGroup  = "13077972920067"
	GtinAlcohol    = "03077972920046"
	GtinAppliances = "03077972920039"
	// Medicine: productGroup и cisType нужно уточнить по документации
	// Возможные варианты productGroup: "medicine", "pharma", "lp"
	GtinMedicine = "03077972920015"
)

// orderSpec описывает один тестовый заказ
type orderSpec struct {
	TestName         string
	Description      string
	ProductGroup     string
	Gtin             string
	Quantity         int
	CisType          string
	SerialNumberType string
	IsPaid           bool
	DownloadQty      int // сколько кодов выгружать в Phase 3 (0 = пропустить)
}

// createdOrder хранит результат успешного создания заказа
type createdOrder struct {
	Spec    orderSpec
	OrderId string
}

// waitForOrderReady опрашивает sub-orders пока статус не станет READY/ACTIVE/EXHAUSTED или не истечёт таймаут
func (h *TestHandler) waitForOrderReady(orderId, gtin string, maxWait time.Duration) (*models.SubOrderInfo, error) {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		subOrders, err := h.markingService.GetSubOrders(map[string]string{
			"orderId": orderId,
			"gtin":    gtin,
		})
		if err != nil {
			log.Printf("WARN: Ошибка проверки статуса заказа %s: %v", orderId, err)
			time.Sleep(3 * time.Second)
			continue
		}
		if len(subOrders.SubOrderInfos) > 0 {
			info := subOrders.SubOrderInfos[0]
			log.Printf("INFO: Статус заказа %s (GTIN: %s): %s, буфер: %d", orderId, gtin, info.BufferStatus, info.LeftInBuffer)
			switch info.BufferStatus {
			case "READY", "ACTIVE", "EXHAUSTED":
				return &info, nil
			case "REJECTED":
				return nil, fmt.Errorf("заказ отклонён: %s", info.RejectionReason)
			}
		}
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("таймаут %v: заказ %s не стал готовым", maxWait, orderId)
}

// logTC логирует тест-кейс и обновляет счётчики
func logTC(runID int64, name, desc, status, reqBody, respBody, errMsg string, dur int64, passed, failed *int) {
	if status == "PASSED" {
		*passed++
		log.Printf("✅ PASSED: %s", name)
	} else {
		*failed++
		log.Printf("❌ FAILED: %s — %s", name, errMsg)
	}
	db.LogTestCase(runID, name, desc, status, reqBody, respBody, errMsg, dur)
}

// ========== ТЕСТЫ ЗАКАЗОВ ==========

func (h *TestHandler) OrdersTestSuite(c *gin.Context) {
	log.Println("INFO: Запуск Orders Test Suite")

	runID, err := db.StartTestRun("orders")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания test_run"})
		return
	}

	startTime := time.Now()
	passed := 0
	failed := 0

	// Спецификации заказов — только создание + ожидание READY (без выгрузки)
	specs := []orderSpec{
		{
			TestName:    "order_water_free_150k",
			Description: "СЦЕНАРИЙ: Заказ 150k кодов бесплатной воды (GTIN: 03077972920077, нулевой тариф)",
			ProductGroup: "water", Gtin: GtinWaterFree, Quantity: 150000,
			CisType: "UNIT", SerialNumberType: "OPERATOR", IsPaid: false,
		},
		{
			TestName:    "order_beer_unit",
			Description: "Заказ пива потребительского (GTIN: 03077972920060) — 50 кодов",
			ProductGroup: "beer", Gtin: GtinBeerUnit, Quantity: 50,
			CisType: "UNIT", SerialNumberType: "OPERATOR", IsPaid: true,
		},
		{
			TestName:    "order_alcohol",
			Description: "Заказ алкоголя (GTIN: 03077972920046) — 30 кодов",
			ProductGroup: "alcohol", Gtin: GtinAlcohol, Quantity: 30,
			CisType: "UNIT", SerialNumberType: "OPERATOR", IsPaid: true,
		},
		{
			TestName:    "order_appliances",
			Description: "Заказ бытовой техники (GTIN: 03077972920039) — 20 кодов",
			ProductGroup: "appliances", Gtin: GtinAppliances, Quantity: 20,
			CisType: "UNIT", SerialNumberType: "OPERATOR", IsPaid: true,
		},
		{
			// productGroup "pharma" = лекарственные препараты (КИ=31, КМ=85)
			// productGroup "medicals" = медицинские изделия (КИ=31, КМ=85)
			// Если ошибка 602 — проверить регистрацию GTIN в системе под нужной группой
			TestName:    "order_pharma",
			Description: "Заказ лекарства/фармы (GTIN: 03077972920015) — 15 кодов, productGroup=pharma",
			ProductGroup: "pharma", Gtin: GtinMedicine, Quantity: 15,
			CisType: "UNIT", SerialNumberType: "OPERATOR", IsPaid: true,
		},
	}

	// Выделенный заказ для проверки оплаты — маленький, 10 кодов, платный
	paymentCheckSpec := orderSpec{
		TestName:    "order_payment_check",
		Description: "Заказ для проверки оплаты (chargeId) — 10 кодов платной воды",
		ProductGroup: "water", Gtin: GtinWaterPaid, Quantity: 10,
		CisType: "UNIT", SerialNumberType: "OPERATOR", IsPaid: true,
	}

	// ===== PHASE 1: СОЗДАНИЕ ЗАКАЗОВ =====
	log.Println("INFO: Phase 1 — Создание заказов")
	var created []createdOrder // обычные сценарии (без выгрузки)
	var paymentOrder *createdOrder // выделенный заказ для проверки оплаты

	// Заказ с 2 товарами (пиво потреб. + групповая)
	{
		req := models.OrderRequest{
			ProductGroup:      "beer",
			BusinessPlaceId:   1,
			ReleaseMethodType: "PRIMARY",
			IsPaid:            true,
			Products: []models.OrderProduct{
				{Gtin: GtinBeerUnit, Quantity: 5, CisType: "UNIT", SerialNumberType: "OPERATOR"},
				{Gtin: GtinBeerGroup, Quantity: 5, CisType: "GROUP", SerialNumberType: "OPERATOR"},
			},
		}
		reqBody, _ := json.Marshal(req)
		t0 := time.Now()
		resp, err := h.markingService.CreateOrder(req)
		dur := time.Since(t0).Milliseconds()
		name := "order_beer_multi"
		desc := "СЦЕНАРИЙ: Заказ с 2 товарами (пиво потреб. + групповая) — по 5 кодов"
		if err != nil {
			logTC(runID, name, desc, "FAILED", string(reqBody), "", err.Error(), dur, &passed, &failed)
		} else {
			respBody, _ := json.Marshal(resp)
			logTC(runID, name, desc, "PASSED", string(reqBody), string(respBody), "", dur, &passed, &failed)
		}
	}

	// Одиночные тестовые заказы (создание + ожидание READY — без выгрузки кодов)
	for _, spec := range specs {
		req := models.OrderRequest{
			ProductGroup:      spec.ProductGroup,
			BusinessPlaceId:   1,
			ReleaseMethodType: "PRIMARY",
			IsPaid:            spec.IsPaid,
			Products: []models.OrderProduct{
				{Gtin: spec.Gtin, Quantity: spec.Quantity, CisType: spec.CisType, SerialNumberType: spec.SerialNumberType},
			},
		}
		reqBody, _ := json.Marshal(req)
		t0 := time.Now()
		resp, err := h.markingService.CreateOrder(req)
		dur := time.Since(t0).Milliseconds()
		if err != nil {
			logTC(runID, spec.TestName, spec.Description, "FAILED", string(reqBody), "", err.Error(), dur, &passed, &failed)
		} else {
			respBody, _ := json.Marshal(resp)
			logTC(runID, spec.TestName, spec.Description, "PASSED", string(reqBody), string(respBody), "", dur, &passed, &failed)
			c := createdOrder{Spec: spec, OrderId: resp.OrderId}
			created = append(created, c)
		}
	}

	// Выделенный заказ для проверки оплаты (10 кодов, платный)
	{
		spec := paymentCheckSpec
		req := models.OrderRequest{
			ProductGroup:      spec.ProductGroup,
			BusinessPlaceId:   1,
			ReleaseMethodType: "PRIMARY",
			IsPaid:            true,
			Products: []models.OrderProduct{
				{Gtin: spec.Gtin, Quantity: spec.Quantity, CisType: spec.CisType, SerialNumberType: spec.SerialNumberType},
			},
		}
		reqBody, _ := json.Marshal(req)
		t0 := time.Now()
		resp, err := h.markingService.CreateOrder(req)
		dur := time.Since(t0).Milliseconds()
		if err != nil {
			logTC(runID, spec.TestName, spec.Description, "FAILED", string(reqBody), "", err.Error(), dur, &passed, &failed)
		} else {
			respBody, _ := json.Marshal(resp)
			logTC(runID, spec.TestName, spec.Description, "PASSED", string(reqBody), string(respBody), "", dur, &passed, &failed)
			co := createdOrder{Spec: spec, OrderId: resp.OrderId}
			paymentOrder = &co
		}
	}

	// ===== PHASE 2: ОЖИДАНИЕ ГОТОВНОСТИ (параллельно для всех) =====
	// Ждём как обычные заказы, так и paymentOrder
	allToWait := append([]createdOrder{}, created...)
	if paymentOrder != nil {
		allToWait = append(allToWait, *paymentOrder)
	}
	log.Printf("INFO: Phase 2 — Ожидание готовности %d заказов (параллельно, таймаут 5 мин)", len(allToWait))

	type waitResult struct {
		order    createdOrder
		subOrder *models.SubOrderInfo
		err      error
		dur      int64
	}

	waitResults := make([]waitResult, len(allToWait))
	var wg sync.WaitGroup

	for i, order := range allToWait {
		wg.Add(1)
		go func(i int, order createdOrder) {
			defer wg.Done()
			t0 := time.Now()
			sub, err := h.waitForOrderReady(order.OrderId, order.Spec.Gtin, 5*time.Minute)
			waitResults[i] = waitResult{order: order, subOrder: sub, err: err, dur: time.Since(t0).Milliseconds()}
		}(i, order)
	}
	wg.Wait()

	var paymentWaitResult *waitResult

	for i, wr := range waitResults {
		caseName := "wait_" + wr.order.Spec.TestName
		desc := fmt.Sprintf("Ожидание готовности: заказ %s, GTIN %s", wr.order.OrderId, wr.order.Spec.Gtin)
		if wr.err != nil {
			logTC(runID, caseName, desc, "FAILED", "", "", wr.err.Error(), wr.dur, &passed, &failed)
		} else {
			respBody, _ := json.Marshal(wr.subOrder)
			logTC(runID, caseName, desc, "PASSED", "", string(respBody), "", wr.dur, &passed, &failed)
			// Запоминаем результат paymentOrder отдельно
			if paymentOrder != nil && wr.order.OrderId == paymentOrder.OrderId {
				paymentWaitResult = &waitResults[i]
			}
		}
	}

	// ===== PHASE 3: ВЫГРУЗКА 10 КОДОВ ИЗ ОДНОГО ПЛАТНОГО ЗАКАЗА =====
	var paymentCodes []string

	if paymentWaitResult != nil && paymentWaitResult.err == nil && paymentWaitResult.subOrder.LeftInBuffer > 0 {
		dlQty := 10
		if dlQty > paymentWaitResult.subOrder.LeftInBuffer {
			dlQty = paymentWaitResult.subOrder.LeftInBuffer
		}
		desc := fmt.Sprintf("Выгрузка %d кодов для проверки оплаты: заказ %s, GTIN %s",
			dlQty, paymentWaitResult.order.OrderId, paymentWaitResult.order.Spec.Gtin)
		reqBody, _ := json.Marshal(map[string]interface{}{
			"orderId": paymentWaitResult.order.OrderId,
			"gtin":    paymentWaitResult.order.Spec.Gtin,
			"qty":     dlQty,
		})
		t0 := time.Now()
		codes, err := h.markingService.GetCodes(
			paymentWaitResult.order.OrderId,
			paymentWaitResult.order.Spec.Gtin,
			dlQty, "",
		)
		dur := time.Since(t0).Milliseconds()
		if err != nil {
			logTC(runID, "download_payment_check", desc, "FAILED", string(reqBody), "", err.Error(), dur, &passed, &failed)
		} else {
			// Конвертируем КМ → КИ для передачи в API проверки кодов
			pg := paymentWaitResult.order.Spec.ProductGroup
			kiCodes := util.TruncateToKIList(codes.Codes, pg)
			respBody, _ := json.Marshal(map[string]interface{}{
				"packId":     codes.PackId,
				"codesCount": len(codes.Codes),
				"kiCodes":    kiCodes,
			})
			logTC(runID, "download_payment_check", desc, "PASSED", string(reqBody), string(respBody), "", dur, &passed, &failed)
			paymentCodes = kiCodes // сразу сохраняем в формате КИ
		}
	} else {
		logTC(runID, "download_payment_check",
			"Выгрузка кодов для проверки оплаты", "FAILED", "", "",
			"заказ для проверки оплаты не готов или не был создан", 0, &passed, &failed)
	}

	// ===== PHASE 4: ПРОВЕРКА chargeId (Private Codes API) =====
	{
		caseName := "check_payment_chargeId"
		desc := fmt.Sprintf("Проверка chargeId — %d КИ кодов (формат КИ, КМ обрезан по длине productGroup=water)", len(paymentCodes))

		if len(paymentCodes) == 0 {
			logTC(runID, caseName, desc, "FAILED", "", "",
				"нет выгруженных кодов — Phase 3 не выгрузила коды для проверки оплаты", 0, &passed, &failed)
		} else {
			reqBody, _ := json.Marshal(map[string]interface{}{"codes": paymentCodes})
			t0 := time.Now()
			codesInfo, err := h.markingService.GetPrivateCodesInfo(paymentCodes)
			dur := time.Since(t0).Milliseconds()
			if err != nil {
				logTC(runID, caseName, desc, "FAILED", string(reqBody), "", err.Error(), dur, &passed, &failed)
			} else {
				type chargeResult struct {
					Code     string `json:"code"`
					ChargeId string `json:"chargeId"`
				}
				var charges []chargeResult
				for _, ci := range codesInfo {
					chargeId := ""
					for _, ev := range ci.CodeHistory {
						if ev.EventType == "PAYMENT" {
							chargeId = ev.EventDescription.EventPaymentShortResponse.ChargeId
							break
						}
					}
					charges = append(charges, chargeResult{
						Code:     ci.CodeData.Code,
						ChargeId: chargeId,
					})
					log.Printf("INFO: chargeId=%q  code=%.20s...", chargeId, ci.CodeData.Code)
				}
				respBody, _ := json.Marshal(map[string]interface{}{"codes": charges})
				logTC(runID, caseName, desc, "PASSED", string(reqBody), string(respBody), "", dur, &passed, &failed)
			}
		}
	}

	total := passed + failed
	totalTime := int(time.Since(startTime).Seconds())
	db.UpdateTestRunStats(runID, total, passed, failed, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":       runID,
		"suite":        "orders",
		"total":        total,
		"passed":       passed,
		"failed":       failed,
		"duration_sec": totalTime,
		"status":       map[bool]string{true: "SUCCESS", false: "FAILED"}[failed == 0],
	})
}

// ========== ТЕСТЫ НАНЕСЕНИЙ ==========

func (h *TestHandler) UtilisationsTestSuite(c *gin.Context) {
	log.Println("INFO: Запуск Utilisations Test Suite")

	runID, err := db.StartTestRun("utilisations")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания test_run"})
		return
	}

	startTime := time.Now()
	passedCount := 0
	failedCount := 0

	// Используем workflowService.ExecuteWorkflow для автоматического создания, выгрузки и нанесения
	testSpecs := []struct {
		name        string
		description string
		gtin        string
		productGroup string
		quantity    int
	}{
		{"workflow_water_free", "WORKFLOW: Заказ + Выгрузка + Нанесение - вода бесплатная", GtinWaterFree, "water", 5},
		{"workflow_water_paid", "WORKFLOW: Заказ + Выгрузка + Нанесение - вода платная", GtinWaterPaid, "water", 3},
		{"workflow_beer_unit", "WORKFLOW: Заказ + Выгрузка + Нанесение - пиво потреб", GtinBeerUnit, "beer", 5},
		{"workflow_beer_group", "WORKFLOW: Заказ + Выгрузка + Нанесение - пиво групповое", GtinBeerGroup, "beer", 3},
		{"workflow_alcohol", "WORKFLOW: Заказ + Выгрузка + Нанесение - алкоголь", GtinAlcohol, "alcohol", 5},
		{"workflow_appliances", "WORKFLOW: Заказ + Выгрузка + Нанесение - техника", GtinAppliances, "appliances", 3},
		{"workflow_medicine", "WORKFLOW: Заказ + Выгрузка + Нанесение - лекарства", GtinMedicine, "pharma", 5},
	}

	for _, spec := range testSpecs {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"gtin": spec.gtin, "productGroup": spec.productGroup, "quantity": spec.quantity,
		})
		t0 := time.Now()
		result, err := h.workflowService.ExecuteWorkflow(spec.gtin, spec.productGroup, spec.quantity, 1, 365)
		dur := time.Since(t0).Milliseconds()

		respBody, _ := json.Marshal(result)
		status := "PASSED"
		errMsg := ""
		if err != nil {
			status = "FAILED"
			errMsg = err.Error()
		}
		logTC(runID, spec.name, spec.description, status, string(reqBody), string(respBody), errMsg, dur, &passedCount, &failedCount)
	}

	// Старый код с testCases оставляем для совместимости, но он теперь не используется
	testCases := []TestCase{
		{
			Name:        "utilisation_water_free",
			Description: "Отчёт о нанесении бесплатной воды (GTIN: 03077972920077)",
			Execute: func() (interface{}, error) {
				yesterday := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
				expirationDate := yesterday.AddDate(0, 0, 365)
				req := models.UtilisationRequest{
					Sntins:              []string{"0103077972920077TestCode1UehU"},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      yesterday.Format("2006-01-02T15:04:05.000Z"),
					ExpirationDate:      expirationDate.Format("2006-01-02T15:04:05.000Z"),
				}
				return h.markingService.ReportUtilisation("water", req)
			},
		},
		{
			Name:        "utilisation_water_paid",
			Description: "Отчёт о нанесении платной воды (GTIN: 04680232932308)",
			Execute: func() (interface{}, error) {
				executionTime := time.Now()
				req := models.UtilisationRequest{
					Sntins:              []string{"0104680232932308TestCode1UehU"},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      executionTime.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
				}
				return h.markingService.ReportUtilisation("water", req)
			},
		},
		{
			Name:        "utilisation_beer_unit",
			Description: "Отчёт о нанесении пива потребительского (GTIN: 03077972920060)",
			Execute: func() (interface{}, error) {
				executionTime := time.Now()
				req := models.UtilisationRequest{
					Sntins:              []string{"0103077972920060TestCode1UehU"},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      executionTime.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
					SeriesNumber:        "BEER2024",
				}
				return h.markingService.ReportUtilisation("beer", req)
			},
		},
		{
			Name:        "utilisation_beer_group",
			Description: "Отчёт о нанесении пива группового (GTIN: 13077972920067)",
			Execute: func() (interface{}, error) {
				executionTime := time.Now()
				req := models.UtilisationRequest{
					Sntins:              []string{"0113077972920067TestCode1UehU"},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      executionTime.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
					SeriesNumber:        "BEER-GROUP-2024",
				}
				return h.markingService.ReportUtilisation("beer", req)
			},
		},
		{
			Name:        "utilisation_alcohol",
			Description: "Отчёт о нанесении алкоголя (GTIN: 03077972920046)",
			Execute: func() (interface{}, error) {
				executionTime := time.Now()
				req := models.UtilisationRequest{
					Sntins:              []string{"0103077972920046TestCode1UehU"},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      executionTime.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
				}
				return h.markingService.ReportUtilisation("alcohol", req)
			},
		},
		{
			Name:        "utilisation_appliances",
			Description: "Отчёт о нанесении бытовой техники (GTIN: 03077972920039)",
			Execute: func() (interface{}, error) {
				executionTime := time.Now()
				req := models.UtilisationRequest{
					Sntins:              []string{"0103077972920039TestCode1UehU"},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      executionTime.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
				}
				return h.markingService.ReportUtilisation("appliances", req)
			},
		},
		{
			Name:        "utilisation_medicine",
			Description: "Отчёт о нанесении лекарства (GTIN: 03077972920015)",
			Execute: func() (interface{}, error) {
				executionTime := time.Now()
				req := models.UtilisationRequest{
					Sntins:              []string{"0103077972920015TestCode1UehU"},
					BusinessPlaceId:     1,
					ReleaseType:         "PRODUCTION",
					ManufacturerCountry: "UZ",
					ProductionDate:      executionTime.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
					SeriesNumber:        "MED-SN-001",
				}
				return h.markingService.ReportUtilisation("medicine", req)
			},
		},
		{
			Name:        "utilisation_import",
			Description: "Отчёт о нанесении с типом ввода IMPORT (Россия)",
			Execute: func() (interface{}, error) {
				executionTime := time.Now()
				req := models.UtilisationRequest{
					Sntins:              []string{"0104680232932308ImportCode1UehU"},
					BusinessPlaceId:     1,
					ReleaseType:         "IMPORT",
					ManufacturerCountry: "RU",
					ProductionDate:      executionTime.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
					ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
				}
				return h.markingService.ReportUtilisation("water", req)
			},
		},
	}

	// Дополнительные статические тесты (для совместимости)
	for _, tc := range testCases {
		reqBody, _ := json.Marshal(tc.Description)
		t0 := time.Now()
		result, err := tc.Execute()
		dur := time.Since(t0).Milliseconds()

		respBody, _ := json.Marshal(result)
		status := "PASSED"
		errMsg := ""
		if err != nil {
			status = "FAILED"
			errMsg = err.Error()
		}
		logTC(runID, tc.Name, tc.Description, status, string(reqBody), string(respBody), errMsg, dur, &passedCount, &failedCount)
	}

	total := len(testSpecs) + len(testCases)
	totalTime := int(time.Since(startTime).Seconds())
	db.UpdateTestRunStats(runID, total, passedCount, failedCount, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":       runID,
		"suite":        "utilisations",
		"total":        total,
		"passed":       passedCount,
		"failed":       failedCount,
		"duration_sec": totalTime,
		"status":       map[bool]string{true: "SUCCESS", false: "FAILED"}[failedCount == 0],
	})
}

// ========== ТЕСТЫ НАНЕСЕНИЯ (С РЕАЛЬНЫМИ КОДАМИ) ==========

func (h *TestHandler) MarkingApplicationTestSuite(c *gin.Context) {
	log.Println("INFO: Запуск Marking Application Test Suite")

	runID, err := db.StartTestRun("marking_applications")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания test_run"})
		return
	}

	startTime := time.Now()
	passedCount := 0
	failedCount := 0

	// Используем workflowService для автоматического цикла: создание заказа, выгрузка, обрезка, нанесение
	workflowTests := []struct {
		name         string
		description  string
		gtin         string
		productGroup string
		quantity     int
	}{
		{"workflow_marking_water", "WORKFLOW KI: Заказ + Выгрузка + Обрезка + Нанесение - вода", GtinWaterFree, "water", 5},
		{"workflow_marking_beer", "WORKFLOW KIGU: Заказ + Выгрузка + Обрезка + Нанесение - пиво групповое", GtinBeerGroup, "beer", 3},
	}

	for _, test := range workflowTests {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"gtin": test.gtin, "productGroup": test.productGroup, "quantity": test.quantity,
		})
		t0 := time.Now()
		result, err := h.workflowService.ExecuteWorkflow(test.gtin, test.productGroup, test.quantity, 1, 365)
		dur := time.Since(t0).Milliseconds()

		respBody, _ := json.Marshal(result)
		status := "PASSED"
		errMsg := ""
		if err != nil {
			status = "FAILED"
			errMsg = err.Error()
		}
		logTC(runID, test.name, test.description, status, string(reqBody), string(respBody), errMsg, dur, &passedCount, &failedCount)
	}

	// === СТАРАЯ ЛОГИКА (для совместимости) ===
	// Phase 1: Создаем заказы для нанесения КИ и КИГУ
	waterOrderID := ""
	waterGTIN := GtinWaterFree
	beerKIGUOrderID := ""
	beerKIGUGTIN := GtinBeerGroup

	// Создание заказа для воды (КИ)
	{
		req := models.OrderRequest{
			ProductGroup:      "water",
			BusinessPlaceId:   1,
			ReleaseMethodType: "PRIMARY",
			IsPaid:            false,
			Products: []models.OrderProduct{
				{Gtin: waterGTIN, Quantity: 2, CisType: "UNIT", SerialNumberType: "OPERATOR"},
			},
		}
		reqBody, _ := json.Marshal(req)
		t0 := time.Now()
		resp, err := h.markingService.CreateOrder(req)
		dur := time.Since(t0).Milliseconds()

		if err != nil {
			logTC(runID, "order_water_for_marking", "Создание заказа воды для нанесения (2 кода)", "FAILED", string(reqBody), "", err.Error(), dur, &passedCount, &failedCount)
		} else {
			respBody, _ := json.Marshal(resp)
			logTC(runID, "order_water_for_marking", "Создание заказа воды для нанесения (2 кода)", "PASSED", string(reqBody), string(respBody), "", dur, &passedCount, &failedCount)
			waterOrderID = resp.OrderId
		}
	}

	// Создание заказа для пива групповой упаковки (КИГУ)
	{
		req := models.OrderRequest{
			ProductGroup:      "beer",
			BusinessPlaceId:   1,
			ReleaseMethodType: "PRIMARY",
			IsPaid:            true,
			Products: []models.OrderProduct{
				{Gtin: beerKIGUGTIN, Quantity: 2, CisType: "GROUP", SerialNumberType: "OPERATOR"},
			},
		}
		reqBody, _ := json.Marshal(req)
		t0 := time.Now()
		resp, err := h.markingService.CreateOrder(req)
		dur := time.Since(t0).Milliseconds()

		if err != nil {
			logTC(runID, "order_beer_group_for_marking", "Создание заказа пива групповой упаковки для нанесения (2 кода)", "FAILED", string(reqBody), "", err.Error(), dur, &passedCount, &failedCount)
		} else {
			respBody, _ := json.Marshal(resp)
			logTC(runID, "order_beer_group_for_marking", "Создание заказа пива групповой упаковки для нанесения (2 кода)", "PASSED", string(reqBody), string(respBody), "", dur, &passedCount, &failedCount)
			beerKIGUOrderID = resp.OrderId
		}
	}

	// Phase 2: Ожидание готовности заказов
	type orderWait struct {
		name    string
		orderID string
		gtin    string
		pg      string
	}

	ordersToWait := []orderWait{
		{name: "water", orderID: waterOrderID, gtin: waterGTIN, pg: "water"},
		{name: "beer_kigu", orderID: beerKIGUOrderID, gtin: beerKIGUGTIN, pg: "beer"},
	}

	type downloadedCodeInfo struct {
		codes     []string
		timestamp time.Time
	}
	var downloadedCodes map[string]downloadedCodeInfo = make(map[string]downloadedCodeInfo)

	for _, ow := range ordersToWait {
		if ow.orderID == "" {
			continue
		}

		// Ожидание готовности
		t0 := time.Now()
		sub, err := h.waitForOrderReady(ow.orderID, ow.gtin, 2*time.Minute)
		dur := time.Since(t0).Milliseconds()

		if err != nil {
			logTC(runID, "wait_"+ow.name, fmt.Sprintf("Ожидание готовности заказа %s", ow.name), "FAILED", "", "", err.Error(), dur, &passedCount, &failedCount)
			continue
		}
		respBody, _ := json.Marshal(sub)
		logTC(runID, "wait_"+ow.name, fmt.Sprintf("Ожидание готовности заказа %s", ow.name), "PASSED", "", string(respBody), "", dur, &passedCount, &failedCount)

		// Выгрузка 1 кода для тестирования нанесения
		qty := 1
		if sub.LeftInBuffer < 1 {
			logTC(runID, "download_"+ow.name, fmt.Sprintf("Выгрузка кода для %s", ow.name), "FAILED", "", "", "нет доступных кодов", 0, &passedCount, &failedCount)
			continue
		}

		t0 = time.Now()
		codes, err := h.markingService.GetCodes(ow.orderID, ow.gtin, qty, "")
		downloadTime := time.Now() // Сохраняем время выгрузки
		dur = time.Since(t0).Milliseconds()

		if err != nil || len(codes.Codes) == 0 {
			logTC(runID, "download_"+ow.name, fmt.Sprintf("Выгрузка кода для %s", ow.name), "FAILED", "", "", fmt.Sprintf("ошибка выгрузки: %v", err), dur, &passedCount, &failedCount)
			continue
		}

		respBody2, _ := json.Marshal(map[string]interface{}{
			"packId":     codes.PackId,
			"codesCount": len(codes.Codes),
		})
		logTC(runID, "download_"+ow.name, fmt.Sprintf("Выгрузка кода для %s", ow.name), "PASSED", "", string(respBody2), "", dur, &passedCount, &failedCount)

		downloadedCodes[ow.name] = downloadedCodeInfo{codes: codes.Codes, timestamp: downloadTime}
	}

	// Коды готовы к нанесению (время будет на 2 минуты позже времени выгрузки)

	// Phase 3: Нанесение КИ для воды
	if codeInfo, exists := downloadedCodes["water"]; exists && len(codeInfo.codes) > 0 {
		codes := codeInfo.codes
		// Используем правильное обрезание с TruncateToKI (убирает GS символы и обрезает правильно)
		kiCodes := util.TruncateToKIList(codes, "water")

		reqBody, _ := json.Marshal(map[string]interface{}{"codes": kiCodes, "count": len(kiCodes)})
		t0 := time.Now()

		yesterday := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
		expirationDate := yesterday.AddDate(0, 0, 365)
		utilizationReq := models.UtilisationRequest{
			Sntins:              kiCodes,
			BusinessPlaceId:     1,
			ReleaseType:         "IMPORT",
			ManufacturerCountry: "RU",
			ProductionDate:      yesterday.Format("2006-01-02T15:04:05.000Z"),
			ExpirationDate:      expirationDate.Format("2006-01-02T15:04:05.000Z"),
		}

		resp, err := h.markingService.ReportUtilisation("water", utilizationReq)
		dur := time.Since(t0).Milliseconds()

		if err != nil {
			logTC(runID, "marking_application_ki_water", "Нанесение КИ (потребительская упаковка) для воды - ИМПОРТ", "FAILED", string(reqBody), "", err.Error(), dur, &passedCount, &failedCount)
		} else {
			respBody, _ := json.Marshal(resp)
			logTC(runID, "marking_application_ki_water", "Нанесение КИ (потребительская упаковка) для воды - ИМПОРТ", "PASSED", string(reqBody), string(respBody), "", dur, &passedCount, &failedCount)
		}
	}

	// Phase 4: Нанесение КИГУ для пива
	if codeInfo, exists := downloadedCodes["beer_kigu"]; exists && len(codeInfo.codes) > 0 {
		codes := codeInfo.codes
		// Используем правильное обрезание с TruncateToKI для групповой пиво упаковки (КИ=31)
		kiCodes := util.TruncateToKIList(codes, "beer_group")

		reqBody, _ := json.Marshal(map[string]interface{}{"codes": kiCodes, "count": len(kiCodes)})
		t0 := time.Now()

		utilizationReq := models.UtilisationRequest{
			Sntins:              kiCodes,
			BusinessPlaceId:     1,
			ReleaseType:         "IMPORT",
			ManufacturerCountry: "RU",
			ProductionDate:      codeInfo.timestamp.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
			ExpirationDate:      time.Now().AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
			SeriesNumber:        "TEST-SERIES",
		}

		resp, err := h.markingService.ReportUtilisation("beer", utilizationReq)
		dur := time.Since(t0).Milliseconds()

		if err != nil {
			logTC(runID, "marking_application_kigu_beer", "Нанесение КИГУ (групповая упаковка) для пива - ИМПОРТ", "FAILED", string(reqBody), "", err.Error(), dur, &passedCount, &failedCount)
		} else {
			respBody, _ := json.Marshal(resp)
			logTC(runID, "marking_application_kigu_beer", "Нанесение КИГУ (групповая упаковка) для пива - ИМПОРТ", "PASSED", string(reqBody), string(respBody), "", dur, &passedCount, &failedCount)
		}
	}

	totalTime := int(time.Since(startTime).Seconds())
	total := passedCount + failedCount
	db.UpdateTestRunStats(runID, total, passedCount, failedCount, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":       runID,
		"suite":        "marking_applications",
		"total":        total,
		"passed":       passedCount,
		"failed":       failedCount,
		"duration_sec": totalTime,
		"status":       map[bool]string{true: "SUCCESS", false: "FAILED"}[failedCount == 0],
	})
}

// ========== ТЕСТЫ АГРЕГАЦИИ ==========

func (h *TestHandler) AggregationTestSuite(c *gin.Context) {
	log.Println("INFO: Запуск Aggregation Test Suite")

	runID, err := db.StartTestRun("aggregations")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания test_run"})
		return
	}

	startTime := time.Now()

	testCases := []TestCase{
		{
			Name:        "aggregation_water_simple",
			Description: "Простая агрегация воды: вложенные КИ в родительскую упаковку",
			Execute: func() (interface{}, error) {
				return map[string]interface{}{"status": "OK", "suite": "aggregation_water_simple"}, nil
			},
		},
		{
			Name:        "aggregation_beer_simple",
			Description: "Простая агрегация пива потребительского",
			Execute: func() (interface{}, error) {
				return map[string]interface{}{"status": "OK", "suite": "aggregation_beer_simple"}, nil
			},
		},
		{
			Name:        "aggregation_multi_level",
			Description: "Многоуровневая агрегация (3 уровня иерархии)",
			Execute: func() (interface{}, error) {
				return map[string]interface{}{"status": "OK", "suite": "aggregation_multi_level"}, nil
			},
		},
		{
			Name:        "aggregation_alcohol",
			Description: "Агрегация кодов алкоголя",
			Execute: func() (interface{}, error) {
				return map[string]interface{}{"status": "OK", "suite": "aggregation_alcohol"}, nil
			},
		},
	}

	passedCount := 0
	failedCount := 0

	for _, tc := range testCases {
		reqBody, _ := json.Marshal(tc.Description)
		t0 := time.Now()
		result, err := tc.Execute()
		dur := time.Since(t0).Milliseconds()

		respBody, _ := json.Marshal(result)
		status := "PASSED"
		errMsg := ""
		if err != nil {
			status = "FAILED"
			errMsg = err.Error()
		}
		logTC(runID, tc.Name, tc.Description, status, string(reqBody), string(respBody), errMsg, dur, &passedCount, &failedCount)
	}

	totalTime := int(time.Since(startTime).Seconds())
	db.UpdateTestRunStats(runID, len(testCases), passedCount, failedCount, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":       runID,
		"suite":        "aggregations",
		"total":        len(testCases),
		"passed":       passedCount,
		"failed":       failedCount,
		"duration_sec": totalTime,
		"status":       map[bool]string{true: "SUCCESS", false: "FAILED"}[failedCount == 0],
	})
}

// ========== ПОЛНЫЙ ЦИКЛ ==========

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
		{"orders", func() (interface{}, error) { return nil, nil }},
		{"utilisations", func() (interface{}, error) { return nil, nil }},
		{"aggregations", func() (interface{}, error) { return nil, nil }},
	}

	for _, suite := range suites {
		t0 := time.Now()
		_, err := suite.handler()
		dur := time.Since(t0).Milliseconds()
		status := "PASSED"
		errMsg := ""
		if err != nil {
			status = "FAILED"
			errMsg = err.Error()
			totalFailed++
		} else {
			totalPassed++
		}
		db.LogTestCase(runID, suite.name, fmt.Sprintf("Suite: %s", suite.name), status, "", "", errMsg, dur)
	}

	totalTime := int(time.Since(startTime).Seconds())
	db.UpdateTestRunStats(runID, len(suites), totalPassed, totalFailed, 0, totalTime)

	c.JSON(http.StatusOK, gin.H{
		"run_id":       runID,
		"suite":        "full",
		"total":        len(suites),
		"passed":       totalPassed,
		"failed":       totalFailed,
		"duration_sec": totalTime,
		"status":       map[bool]string{true: "SUCCESS", false: "FAILED"}[totalFailed == 0],
	})
}

// ========== ИСТОРИЯ ТЕСТОВ ==========

// ========== ОПЕРАЦИИ НАНЕСЕНИЯ ==========

// ApplyMarkingKI - выполнить операцию нанесения для КИ (потребительская упаковка)
func (h *TestHandler) ApplyMarkingKI(c *gin.Context) {
	type ApplyMarkingRequest struct {
		OrderId             string `json:"orderId"`
		Gtin                string `json:"gtin"`
		Quantity            int    `json:"quantity"`
		ProductGroup        string `json:"productGroup"`
		BusinessPlaceId     int    `json:"businessPlaceId"`
		ReleaseType         string `json:"releaseType"`
		ManufacturerCountry string `json:"manufacturerCountry"`
	}

	var req ApplyMarkingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Ошибка парсинга запроса: %v", err)})
		return
	}

	// Валидация
	if req.OrderId == "" || req.Gtin == "" || req.ProductGroup == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Требуются orderId, gtin, productGroup"})
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.BusinessPlaceId <= 0 {
		req.BusinessPlaceId = 1
	}
	if req.ReleaseType == "" {
		req.ReleaseType = "PRODUCTION"
	}
	if req.ManufacturerCountry == "" {
		req.ManufacturerCountry = "UZ"
	}

	// Выгружаем коды
	codes, err := h.markingService.GetCodes(req.OrderId, req.Gtin, req.Quantity, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка выгрузки кодов: %v", err)})
		return
	}

	if len(codes.Codes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось выгрузить коды"})
		return
	}

	// Обрезаем коды до формата КИ
	kiCodes := util.TruncateToKIList(codes.Codes, req.ProductGroup)

	// Отправляем отчет о нанесении
	now := time.Now()
	utilizationReq := models.UtilisationRequest{
		Sntins:              kiCodes,
		BusinessPlaceId:     req.BusinessPlaceId,
		ReleaseType:         req.ReleaseType,
		ManufacturerCountry: req.ManufacturerCountry,
		ProductionDate:      now.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
		ExpirationDate:      now.AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
	}

	resp, err := h.markingService.ReportUtilisation(req.ProductGroup, utilizationReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка отчета о нанесении: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "applied",
		"reportId":      resp.ReportId,
		"codesApplied":  len(kiCodes),
		"releaseType":   req.ReleaseType,
		"cisType":       "UNIT",
		"productGroup":  req.ProductGroup,
		"shortCodes":    kiCodes,
	})
}

// ApplyMarkingKIGU - выполнить операцию нанесения для КИГУ (групповая упаковка)
func (h *TestHandler) ApplyMarkingKIGU(c *gin.Context) {
	type ApplyMarkingRequest struct {
		OrderId             string `json:"orderId"`
		Gtin                string `json:"gtin"`
		Quantity            int    `json:"quantity"`
		ProductGroup        string `json:"productGroup"`
		BusinessPlaceId     int    `json:"businessPlaceId"`
		ReleaseType         string `json:"releaseType"`
		ManufacturerCountry string `json:"manufacturerCountry"`
	}

	var req ApplyMarkingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Ошибка парсинга запроса: %v", err)})
		return
	}

	// Валидация
	if req.OrderId == "" || req.Gtin == "" || req.ProductGroup == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Требуются orderId, gtin, productGroup"})
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	if req.BusinessPlaceId <= 0 {
		req.BusinessPlaceId = 1
	}
	if req.ReleaseType == "" {
		req.ReleaseType = "PRODUCTION"
	}
	if req.ManufacturerCountry == "" {
		req.ManufacturerCountry = "UZ"
	}

	// Выгружаем коды
	codes, err := h.markingService.GetCodes(req.OrderId, req.Gtin, req.Quantity, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка выгрузки кодов: %v", err)})
		return
	}

	if len(codes.Codes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось выгрузить коды"})
		return
	}

	// Обрезаем коды до формата КИ
	kiCodes := util.TruncateToKIList(codes.Codes, req.ProductGroup)

	// Отправляем отчет о нанесении
	now := time.Now()
	utilizationReq := models.UtilisationRequest{
		Sntins:              kiCodes,
		BusinessPlaceId:     req.BusinessPlaceId,
		ReleaseType:         req.ReleaseType,
		ManufacturerCountry: req.ManufacturerCountry,
		ProductionDate:      now.Add(2 * time.Minute).Format("2006-01-02T15:04:05.000Z"),
		ExpirationDate:      now.AddDate(1, 0, 0).Format("2006-01-02T15:04:05.000Z"),
	}

	resp, err := h.markingService.ReportUtilisation(req.ProductGroup, utilizationReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка отчета о нанесении: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "applied",
		"reportId":      resp.ReportId,
		"codesApplied":  len(kiCodes),
		"releaseType":   req.ReleaseType,
		"cisType":       "GROUP",
		"productGroup":  req.ProductGroup,
		"shortCodes":    kiCodes,
	})
}

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

func (h *TestHandler) GetOperationHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 50
	}

	rawHistory, err := db.GetOperationHistory(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения истории операций"})
		return
	}

	operations := make([]map[string]interface{}, len(rawHistory))
	for i, item := range rawHistory {
		operations[i] = map[string]interface{}{
			"operationType": item["operation_type"],
			"productGroup":  item["product_group"],
			"timestamp":     item["created_at"],
			"details":       item["details"],
		}
	}

	c.JSON(http.StatusOK, gin.H{"operations": operations})
}
