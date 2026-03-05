// internal/api/marking_handler.go
package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"api_tester/internal/models"
	"api_tester/internal/service" // Импортируем наш сервис

	"github.com/gin-gonic/gin"
)

// MarkingHandler определяет методы для обработки запросов, связанных с маркировкой
type MarkingHandler struct {
	markingService service.MarkingService // Зависимость от интерфейса сервиса
}

// NewMarkingHandler создает новый экземпляр MarkingHandler
func NewMarkingHandler(s service.MarkingService) *MarkingHandler {
	return &MarkingHandler{
		markingService: s,
	}
}

func (h *MarkingHandler) GetPublicCodesInfo(c *gin.Context) {
	var req models.PublicCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("ERROR: Некорректный JSON-запрос: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос", "details": err.Error()})
		return
	}

	if len(req.Codes) == 0 {
		log.Println("WARNING: Запрос пришел с пустым массивом кодов.")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Массив кодов маркировки не может быть пустым"})
		return
	}

	log.Printf("INFO: Получен запрос на получение публичной инфо о КМ для %d кодов.", len(req.Codes))

	// Вызываем наш MarkingService
	codesInfo, err := h.markingService.GetPublicCodesInfo(req.Codes)
	if err != nil {
		log.Printf("ERROR: Ошибка при получении информации о КМ: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении информации о кодах маркировки", "details": err.Error()})
		return
	}

	log.Printf("INFO: Успешно обработан запрос на публичную инфо о КМ.")
	c.JSON(http.StatusOK, codesInfo)
}

func (h *MarkingHandler) CreateOrder(c *gin.Context) {
	var req models.OrderRequest
	// Привязываем JSON из запроса к структуре
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("ERROR: Ошибка парсинга заказа: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный формат заказа"})
		return
	}

	// Вызываем сервис
	res, err := h.markingService.CreateOrder(req)
	if err != nil {
		log.Printf("ERROR: Не удалось создать заказ: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Возвращаем OrderId клиенту
	c.JSON(http.StatusOK, res)
}
func (h *MarkingHandler) GetOrders(c *gin.Context) {
	// 1. Собираем фильтры из запроса пользователя
	filters := make(map[string]string)

	// Gin позволяет достать параметры из URL: /orders?status=READY
	filters["orderId"] = c.Query("orderId")
	filters["status"] = c.Query("status")
	filters["productGroup"] = c.Query("productGroup")
	filters["dateFrom"] = c.Query("dateFrom")
	filters["dateTo"] = c.Query("dateTo")
	filters["limit"] = c.DefaultQuery("limit", "100") // Если лимит не указан, ставим 100

	// 2. Вызываем сервис
	res, err := h.markingService.GetOrders(filters)
	if err != nil {
		log.Printf("ERROR: Не удалось получить список заказов: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. Возвращаем результат
	c.JSON(http.StatusOK, res)
}

func (h *MarkingHandler) GetCodes(c *gin.Context) {
	orderId := c.Query("orderId")
	gtin := c.Query("gtin")
	quantityStr := c.Query("quantity")
	lastPackId := c.Query("lastPackId")

	if orderId == "" || gtin == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "orderId и gtin обязательны"})
		return
	}

	quantity, _ := strconv.Atoi(quantityStr)
	if quantity <= 0 {
		quantity = 1 // Значение по умолчанию
	}

	res, err := h.markingService.GetCodes(orderId, gtin, quantity, lastPackId)
	if err != nil {
		log.Printf("ERROR: Не удалось выгрузить коды: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *MarkingHandler) GetSubOrders(c *gin.Context) {
	filters := make(map[string]string)
	filters["orderId"] = c.Query("orderId")
	filters["gtin"] = c.Query("gtin")
	filters["status"] = c.Query("status")

	res, err := h.markingService.GetSubOrders(filters)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, res)
}

func (h *MarkingHandler) ReportUtilisation(c *gin.Context) {
	productGroup := c.Query("productGroup")
	if productGroup == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "параметр productGroup обязателен"})
		return
	}

	// 1. Читаем "сырое" тело запроса (байты)
	bodyBytes, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "не удалось прочитать тело запроса"})
		return
	}

	// 2. Экранируем невидимый символ \x1d (GS), заменяя его на строку \u001d
	// Это делает JSON валидным для парсинга, но СОХРАНЯЕТ символ для отправки в ASL
	sanitizedBody := bytes.ReplaceAll(bodyBytes, []byte{0x1d}, []byte(`\u001d`))

	var req models.UtilisationRequest
	// 3. Декодируем уже исправленный (валидный JSON) в структуру
	if err := json.Unmarshal(sanitizedBody, &req); err != nil {
		log.Printf("ERROR: JSON Bind Error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный формат JSON", "details": err.Error()})
		return
	}

	// 4. Вызываем сервис (внутри сервиса мы уже используем SetEscapeHTML(false),
	// так что символ уйдет в ASL в первозданном виде)
	res, err := h.markingService.ReportUtilisation(productGroup, req)
	if err != nil {
		log.Printf("ERROR: Ошибка при подаче отчета: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
