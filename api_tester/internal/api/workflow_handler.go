package api

import (
	"api_tester/internal/models"
	"api_tester/internal/service"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type WorkflowHandler struct {
	workflowService *service.WorkflowService
}

func NewWorkflowHandler(ws *service.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{workflowService: ws}
}

// ExecuteWorkflow - ГЛАВНЫЙ ENDPOINT: Пользователь передает ТОЛЬКО: gtin, productGroup, quantity
// ВСЕ остальное происходит автоматически (Заказ → Выгрузка → Обрезка → Нанесение)
func (h *WorkflowHandler) ExecuteWorkflow(c *gin.Context) {
	var req struct {
		Gtin            string `json:"gtin" binding:"required"`
		ProductGroup    string `json:"productGroup" binding:"required"`
		Quantity        int    `json:"quantity" binding:"required"`
		BusinessPlaceId int    `json:"businessPlaceId"`
		ExpirationDays  int    `json:"expirationDays"` // Дней для срока годности (по умолчанию 365)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "требуются поля: gtin, productGroup, quantity",
			"details": err.Error(),
		})
		return
	}

	// По умолчанию businessPlaceId = 1 и expirationDays = 365 (1 год)
	if req.BusinessPlaceId == 0 {
		req.BusinessPlaceId = 1
	}
	if req.ExpirationDays == 0 {
		req.ExpirationDays = 365 // 1 год по умолчанию
	}

	log.Printf("🚀 Получен запрос workflow: GTIN=%s, Group=%s, Qty=%d, BP=%d, ExpirationDays=%d", req.Gtin, req.ProductGroup, req.Quantity, req.BusinessPlaceId, req.ExpirationDays)

	result, err := h.workflowService.ExecuteWorkflow(req.Gtin, req.ProductGroup, req.Quantity, req.BusinessPlaceId, req.ExpirationDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":                "success",
		"report_id":             result.ReportId,
		"codes_for_aggregation": result.CodesForAggregation,
	})
}

// CompleteWorkflow - ВСЕ В ОДНОМ: Заказ → Выгрузка → Обрезка → Нанесение
// Принимает только OrderRequest и кол-во кодов для нанесения - всё остальное автоматически
func (h *WorkflowHandler) CompleteWorkflow(c *gin.Context) {
	var req struct {
		// Данные заказа
		ProductGroup      string                `json:"productGroup" binding:"required"`
		BusinessPlaceId   int                   `json:"businessPlaceId"`
		ReleaseMethodType string                `json:"releaseMethodType" binding:"required"`
		IsPaid            bool                  `json:"isPaid"`
		Products          []models.OrderProduct `json:"products" binding:"required"`
		// Параметры для нанесения
		UtilizationQuantity int `json:"utilizationQuantity"` // кол-во кодов для нанесения (если 0 - все)
		ExpirationDays      int `json:"expirationDays"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "некорректное тело запроса",
			"details": err.Error(),
		})
		return
	}

	// Конвертируем в OrderRequest
	orderReq := models.OrderRequest{
		ProductGroup:      req.ProductGroup,
		BusinessPlaceId:   req.BusinessPlaceId,
		ReleaseMethodType: req.ReleaseMethodType,
		IsPaid:            req.IsPaid,
		Products:          req.Products,
	}

	// Если BusinessPlaceId не указан, ставим по умолчанию = 1
	if orderReq.BusinessPlaceId == 0 {
		orderReq.BusinessPlaceId = 1
	}

	// Запускаем полный workflow
	result, err := h.workflowService.CompleteWorkflow(orderReq, req.UtilizationQuantity, req.ExpirationDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":                "success",
		"report_id":             result.ReportId,
		"codes_for_aggregation": result.CodesForAggregation,
	})
}

// CreateOrderAndRunFullCycle - Создает заказ и запускает цикл
// Может работать с query параметрами ИЛИ JSON body
func (h *WorkflowHandler) CreateOrderAndRunFullCycle(c *gin.Context) {
	var orderReq models.OrderRequest
	if err := c.ShouldBindJSON(&orderReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "некорректное тело запроса",
			"details": err.Error(),
		})
		return
	}

	// Получаем параметры из query для цикла
	gtin := c.Query("gtin")
	productGroup := c.Query("productGroup")
	quantity, _ := strconv.Atoi(c.DefaultQuery("quantity", "1"))
	businessPlaceId, _ := strconv.Atoi(c.DefaultQuery("businessPlaceId", "1"))

	// Валидация
	if gtin == "" || productGroup == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "параметры gtin и productGroup обязательны",
		})
		return
	}

	// Вызываем сервис для создания заказа и запуска цикла
	workflowResponse, err := h.workflowService.CreateOrderAndRunCycle(orderReq, gtin, productGroup, quantity, businessPlaceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":                "success",
		"report_id":             workflowResponse.ReportId,
		"codes_for_aggregation": workflowResponse.CodesForAggregation,
	})
}

// RunFullCycle обрабатывает запрос на запуск полной цепочки: Ожидание -> Выгрузка -> Обрезка -> Нанесение
// Может работать с query параметрами ИЛИ JSON body
func (h *WorkflowHandler) RunFullCycle(c *gin.Context) {
	type RunRequest struct {
		OrderId         string `json:"orderId"`
		Gtin            string `json:"gtin"`
		ProductGroup    string `json:"productGroup"`
		Quantity        int    `json:"quantity,omitempty"`
		BusinessPlaceId int    `json:"businessPlaceId,omitempty"`
	}

	// Инициализируем переменные по умолчанию
	orderId := ""
	gtin := ""
	productGroup := ""
	quantity := 1
	businessPlaceId := 1

	// 1. Читаем из query параметров (приоритет 1)
	if queryOrderId := c.Query("orderId"); queryOrderId != "" {
		orderId = queryOrderId
	}
	if queryGtin := c.Query("gtin"); queryGtin != "" {
		gtin = queryGtin
	}
	if queryProductGroup := c.Query("productGroup"); queryProductGroup != "" {
		productGroup = queryProductGroup
	}
	if queryQty := c.Query("quantity"); queryQty != "" {
		quantity, _ = strconv.Atoi(queryQty)
	}
	if queryBp := c.Query("businessPlaceId"); queryBp != "" {
		businessPlaceId, _ = strconv.Atoi(queryBp)
	}

	// 2. Если в query пусто, читаем из JSON body (приоритет 2)
	if orderId == "" || gtin == "" || productGroup == "" {
		var req RunRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			if req.OrderId != "" {
				orderId = req.OrderId
			}
			if req.Gtin != "" {
				gtin = req.Gtin
			}
			if req.ProductGroup != "" {
				productGroup = req.ProductGroup
			}
			if req.Quantity > 0 {
				quantity = req.Quantity
			}
			if req.BusinessPlaceId > 0 {
				businessPlaceId = req.BusinessPlaceId
			}
		}
	}

	// 3. Валидация обязательных полей
	if orderId == "" || gtin == "" || productGroup == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "параметры orderId, gtin и productGroup обязательны. Передайте их либо в query, либо в JSON body",
		})
		return
	}

	// 4. Вызываем наш сервис
	utilRes, err := h.workflowService.RunFullCycle(orderId, gtin, productGroup, quantity, businessPlaceId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	workflowResponse := &models.WorkflowResponse{
		ReportId:            utilRes.ReportId,
		CodesForAggregation: []string{}, // TODO: Заполнить codesForAggregation
	}

	c.JSON(http.StatusOK, gin.H{
		"status":                "success",
		"report_id":             workflowResponse.ReportId,
		"codes_for_aggregation": workflowResponse.CodesForAggregation,
	})
}

// ReportAggregation - Подача отчета об агрегации маркированных товаров
// POST /v1/workflow/report-aggregation
// Принимает: aggregationUnits, businessPlaceId, documentDate, productionOrderId (опционально)
// Возвращает: documentId зарегистрированного отчета
func (h *WorkflowHandler) ReportAggregation(c *gin.Context) {
	var doc models.AggregationDocument

	if err := c.ShouldBindJSON(&doc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "некорректное тело запроса",
			"details": err.Error(),
		})
		return
	}

	// Если businessPlaceId не указан, ставим по умолчанию = 1
	if doc.BusinessPlaceId == 0 {
		doc.BusinessPlaceId = 1
	}

	// Если документата не указана, ставим текущее время (ISO 8601)
	if doc.DocumentDate == "" {
		doc.DocumentDate = "2006-01-02T15:04:05Z"
	}

	log.Printf("📦 Получен запрос агрегации: %d упаковок, BP=%d", len(doc.AggregationUnits), doc.BusinessPlaceId)

	result, err := h.workflowService.ReportAggregation(doc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"document_id": result.DocumentId,
	})
}
