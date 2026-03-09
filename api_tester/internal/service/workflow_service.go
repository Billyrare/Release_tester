package service

import (
	"api_tester/internal/db"
	"api_tester/internal/logger"
	"api_tester/internal/models"
	"api_tester/internal/util"
	"fmt"
	"time"
)

type WorkflowService struct {
	markingService MarkingService
}

func NewWorkflowService(ms MarkingService) *WorkflowService {
	return &WorkflowService{markingService: ms}
}

// ExecuteWorkflow - ГЛАВНЫЙ МЕТОД: Пользователь передает только GTIN, Group, Quantity - ВСЕ остальное автоматически
// Заказ → Выгрузка → Обрезка → Нанесение
func (w *WorkflowService) ExecuteWorkflow(gtin string, productGroup string, quantity int, businessPlaceId int, expirationDays int) (*models.WorkflowResponse, error) {
	var codesForAggregation []string
	logger.GetLogger().Infof("WORKFLOW: 🚀 Запуск workflow для GTIN %s, группа %s, кол-во %d, expirationDays: %d", gtin, productGroup, quantity, expirationDays)

	// 1. Создаем заказ с ПРАВИЛЬНЫМИ параметрами
	orderReq := models.OrderRequest{
		ProductGroup:      productGroup,
		BusinessPlaceId:   businessPlaceId,
		ReleaseMethodType: "PRIMARY", // Правильный тип выпуска
		IsPaid:            true,
		Products: []models.OrderProduct{
			{
				Gtin:             gtin,
				Quantity:         quantity,
				CisType:          "UNIT", // Правильный тип CIS
				SerialNumberType: "OPERATOR",
			},
		},
	}

	logger.GetLogger().Infof("WORKFLOW: 📦 Создание заказа с %d кодами для GTIN %s", quantity, gtin)
	orderRes, err := w.markingService.CreateOrder(orderReq)
	if err != nil {
		db.LogOperation("WORKFLOW", productGroup, "N/A", "ERROR", "Ошибка создания заказа: "+err.Error())
		return nil, fmt.Errorf("ошибка создания заказа: %w", err)
	}
	orderId := orderRes.OrderId
	logger.GetLogger().Infof("WORKFLOW: ✅ Заказ создан: %s", orderId)

	// 2. Ожидание готовности и выгрузка полных кодов (КМ)
	logger.GetLogger().Infof("WORKFLOW: ⏳ Ожидание готовности кодов (~45 сек)...")
	fullCodesResponse, err := w.collectCodesWithRetry(orderId, gtin, quantity)
	if err != nil {
		db.LogOperation("WORKFLOW", productGroup, orderId, "ERROR", "Ошибка сбора кодов: "+err.Error())
		return nil, err
	}
	logger.GetLogger().Infof("WORKFLOW: ✅ Получено %d полных кодов", len(fullCodesResponse.Codes))

	// 3. Подготовка кодов (обрезка)
	codesForUtilisation := fullCodesResponse.Codes
	logger.GetLogger().Infof("WORKFLOW: 📌 Группа %s. Используем коды как есть", productGroup)

	// 4. Формируем запрос на нанесение (Utilisation)
	// ProductionDate = вчера (в прошлом)
	// ExpirationDate = ProductionDate + дни
	yesterday := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
	productionDate := yesterday
	expirationDate := time.Now().Truncate(24*time.Hour).AddDate(0, 0, expirationDays)

	prodDateStr := productionDate.Format("2006-01-02T15:04:05.000Z")
	expDateStr := expirationDate.Format("2006-01-02T15:04:05.000Z")
	logger.GetLogger().Infof("WORKFLOW: 📅 ProductionDate: %s (yesterday), ExpirationDate: %s (today + %d days)", prodDateStr, expDateStr, expirationDays)

	utilReq := models.UtilisationRequest{
		Sntins:              codesForUtilisation,
		BusinessPlaceId:     businessPlaceId,
		ManufacturerCountry: "UZ",
		ReleaseType:         "PRODUCTION",
		ProductionDate:      prodDateStr,
		ExpirationDate:      expDateStr,
	}

	// 5. Отправляем отчет о нанесении (батчами)
	logger.GetLogger().Infof("WORKFLOW: 📤 Отправка отчета о нанесении для %d кодов...", len(codesForUtilisation))
	utilRes, err := w.markingService.ReportUtilisationInBatches(productGroup, utilReq, 1000)
	if err != nil {
		db.LogOperation("WORKFLOW", productGroup, orderId, "ERROR", "Ошибка нанесения: "+err.Error())
		return nil, fmt.Errorf("ошибка при подаче отчета о нанесении: %w", err)
	}

	// Обрезаем коды для appliances после нанесения

	if productGroup == "appliances" {
		logger.GetLogger().Infof("WORKFLOW: ✂️  Группа appliances. Обрезаем %d кодов (92 → 31-38 символов) для агрегации...", len(codesForUtilisation))
		codesForAggregation = util.ConvertToKIList(codesForUtilisation)
		logger.GetLogger().Infof("WORKFLOW: ℹ️ Коды для агрегации: %v", codesForAggregation) // Добавлено логирование
	} else {
		codesForAggregation = codesForUtilisation
	}
	if productGroup == "appliances" {
		logger.GetLogger().Infof("WORKFLOW: ℹ️ Коды для агрегации: %v", codesForAggregation) // Добавлено логирование
	}
	logger.GetLogger().Infof("WORKFLOW: ✅✅✅ WORKFLOW УСПЕШНО ЗАВЕРШЕН! ReportID: %s", utilRes.ReportId)
	return &models.WorkflowResponse{
		ReportId:            utilRes.ReportId,
		CodesForAggregation: codesForAggregation,
	}, nil
}

// CompleteWorkflow - ВСЕ В ОДНОМ: Создание заказа -> Ожидание -> Выгрузка → Обрезка → Нанесение
func (w *WorkflowService) CompleteWorkflow(orderReq models.OrderRequest, utilizationQuantity int, expirationDays int) (*models.WorkflowResponse, error) {
	productGroup := orderReq.ProductGroup
	var codesForAggregation []string
	logger.GetLogger().Infof("WORKFLOW: Запуск полного workflow для группы %s", productGroup)

	// 1. Извлекаем GTIN из первого продукта
	if len(orderReq.Products) == 0 {
		return nil, fmt.Errorf("в заказе должно быть хотя бы одно изделие")
	}
	gtin := orderReq.Products[0].Gtin
	quantityForOrder := orderReq.Products[0].Quantity

	// 2. Создаем заказ
	logger.GetLogger().Infof("WORKFLOW: Создание заказа с %d кодами для GTIN %s", quantityForOrder, gtin)
	orderRes, err := w.markingService.CreateOrder(orderReq)
	if err != nil {
		db.LogOperation("WORKFLOW", orderReq.ProductGroup, "N/A", "ERROR", "Ошибка создания заказа: "+err.Error())
		return nil, fmt.Errorf("ошибка создания заказа: %w", err)
	}
	orderId := orderRes.OrderId
	logger.GetLogger().Infof("WORKFLOW: ✅ Заказ создан: %s", orderId)

	// 3. Ожидание готовности и выгрузка полных кодов (КМ)
	logger.GetLogger().Infof("WORKFLOW: Ожидание готовности кодов для %s...", orderId)
	fullCodesResponse, err := w.collectCodesWithRetry(orderId, gtin, quantityForOrder)

	logger.GetLogger().Infof("WORKFLOW: Получено %d полных кодов", len(fullCodesResponse.Codes))

	// 4. Определяем кол-во кодов для нанесения (если не указано, берем все)
	codesToUse := fullCodesResponse.Codes
	if utilizationQuantity > 0 && utilizationQuantity < len(fullCodesResponse.Codes) {
		codesToUse = fullCodesResponse.Codes[:utilizationQuantity]
		logger.GetLogger().Infof("WORKFLOW: Используем %d из %d кодов для нанесения", utilizationQuantity, len(fullCodesResponse.Codes))
	}

	// 5. Подготовка кодов (обрезка для appliances)
	var codesForUtilisation []string
	if orderReq.ProductGroup == "appliances" {
		logger.GetLogger().Infof("WORKFLOW: Группа appliances. Обрезаем %d кодов (92 -> 31-38 симв)...", len(codesToUse))
		codesForUtilisation = util.ConvertToKIList(codesToUse)
	} else {
		codesForUtilisation = codesToUse
	}

	// 6. Формируем запрос на нанесение (Utilisation)
	// ProductionDate = вчера (в прошлом)
	// ExpirationDate = ProductionDate + дни
	yesterday := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
	productionDate := yesterday
	expirationDate := time.Now().Truncate(24*time.Hour).AddDate(0, 0, expirationDays)

	prodDateStr := productionDate.Format("2006-01-02T15:04:05.000Z")
	expDateStr := expirationDate.Format("2006-01-02T15:04:05.000Z")
	logger.GetLogger().Infof("WORKFLOW: 📅 Исправленный формат дат: Prod: %s, Exp: %s", prodDateStr, expDateStr)

	utilReq := models.UtilisationRequest{
		Sntins:              codesForUtilisation,
		BusinessPlaceId:     orderReq.BusinessPlaceId,
		ManufacturerCountry: "UZ",
		ReleaseType:         "PRODUCTION",
		ProductionDate:      prodDateStr,
		ExpirationDate:      expDateStr,
	}

	// 7. Отправляем отчет о нанесении (батчами)
	logger.GetLogger().Infof("WORKFLOW: Отправка отчета о нанесении для %d кодов...", len(codesForUtilisation))
	utilRes, err := w.markingService.ReportUtilisationInBatches(orderReq.ProductGroup, utilReq, 1000)
	if err != nil {
		db.LogOperation("WORKFLOW", orderReq.ProductGroup, orderId, "ERROR", "Ошибка нанесения: "+err.Error())
		return nil, fmt.Errorf("ошибка при подаче отчета о нанесении: %w", err)
	}

	db.LogOperation("WORKFLOW", orderReq.ProductGroup, orderId, "SUCCESS", fmt.Sprintf("Полный workflow завершен. Нанесено %d кодов. ReportID: %s", len(codesForUtilisation), utilRes.ReportId))
	if productGroup == "appliances" {
		logger.GetLogger().Infof("WORKFLOW: ℹ️ Коды для агрегации: %v", codesForAggregation) // Добавлено логирование
	}
	logger.GetLogger().Infof("WORKFLOW: ✅ Полный workflow завершен! ReportID: %s", utilRes.ReportId)
	return &models.WorkflowResponse{
		ReportId:            utilRes.ReportId,
		CodesForAggregation: codesForAggregation,
	}, nil
}

// CreateOrderAndRunCycle - Создает заказ и сразу запускает полный цикл
func (w *WorkflowService) CreateOrderAndRunCycle(orderReq models.OrderRequest, gtin string, productGroup string, quantity int, businessPlaceId int) (*models.WorkflowResponse, error) {
	logger.GetLogger().Infof("WORKFLOW: Создание заказа и запуск полного цикла для группа %s", productGroup)

	// 1. Создаем заказ
	orderRes, err := w.markingService.CreateOrder(orderReq)
	if err != nil {
		db.LogOperation("WORKFLOW", productGroup, "N/A", "ERROR", "Ошибка при создании заказа: "+err.Error())
		return nil, fmt.Errorf("ошибка при создании заказа: %w", err)
	}

	logger.GetLogger().Infof("WORKFLOW: Заказ успешно создан с ID: %s", orderRes.OrderId)

	// 2. Запускаем полный цикл с новым orderId
	utilRes, err := w.RunFullCycle(orderRes.OrderId, gtin, productGroup, quantity, businessPlaceId)
	if err != nil {
		db.LogOperation("WORKFLOW", productGroup, orderRes.OrderId, "ERROR", "Ошибка при выполнении цикла: "+err.Error())
		return nil, err
	}

	workflowResponse := &models.WorkflowResponse{
		ReportId:            utilRes.ReportId,
		CodesForAggregation: []string{}, // TODO: Заполнить codesForAggregation
	}

	logger.GetLogger().Infof("WORKFLOW: Заказ %s успешно обработан полностью", orderRes.OrderId)
	return workflowResponse, nil
}

// RunFullCycle - Автоматическая цепочка: Ожидание -> Выгрузка → Обрезка → Нанесение
func (w *WorkflowService) RunFullCycle(orderId, gtin string, productGroup string, quantity int, businessPlaceId int) (*models.UtilisationResponse, error) {
	logger.GetLogger().Infof("WORKFLOW: Запуск полного цикла для заказа %s", orderId)

	// 1. Ожидание готовности и выгружка полных кодов (КМ)
	fullCodesResponse, err := w.collectCodesWithRetry(orderId, gtin, quantity)
	if err != nil {
		db.LogOperation("WORKFLOW", productGroup, orderId, "ERROR", "Ошибка сбора кодов: "+err.Error())
		return nil, err
	}

	// 2. Подготовка кодов для нанесения (КИ)
	var codesForUtilisation []string
	if productGroup == "appliances" {
		logger.GetLogger().Infof("WORKFLOW: Группа appliances. Обрезаем %d кодов (92 -> 31-38 симв)...", len(fullCodesResponse.Codes))
		codesForUtilisation = util.ConvertToKIList(fullCodesResponse.Codes)
	} else {
		codesForUtilisation = fullCodesResponse.Codes
	}

	// 3. Формируем запрос на нанесение (Utilisation)
	// ProductionDate = вчера (в прошлом)
	// ExpirationDate = ProductionDate + 365 дней
	yesterday := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1)
	productionDate := yesterday
	expirationDate := yesterday.AddDate(0, 0, 365)

	prodDateStr := productionDate.Format("2006-01-02T15:04:05.000Z")
	expDateStr := expirationDate.Format("2006-01-02T15:04:05.000Z")
	logger.GetLogger().Infof("WORKFLOW: 📅 ProductionDate: %s (yesterday), ExpirationDate: %s (+365 days)", prodDateStr, expDateStr)

	utilReq := models.UtilisationRequest{
		Sntins:              codesForUtilisation,
		BusinessPlaceId:     businessPlaceId,
		ManufacturerCountry: "UZ",
		ReleaseType:         "PRODUCTION",
		ProductionDate:      prodDateStr,
		ExpirationDate:      expDateStr,
	}

	// 4. Отправляем отчет о нанесении (батчами)
	logger.GetLogger().Infof("WORKFLOW: Отправка отчета о нанесении для %d кодов...", len(codesForUtilisation))
	utilRes, err := w.markingService.ReportUtilisationInBatches(productGroup, utilReq, 1000)
	if err != nil {
		db.LogOperation("WORKFLOW", productGroup, orderId, "ERROR", "Ошибка нанесения: "+err.Error())
		return nil, fmt.Errorf("ошибка при подаче отчета о нанесении: %w", err)
	}

	db.LogOperation("WORKFLOW", productGroup, utilRes.ReportId, "SUCCESS", fmt.Sprintf("Цикл завершен. Нанесено %d кодов", len(codesForUtilisation)))

	logger.GetLogger().Infof("WORKFLOW: Цепочка успешно завершена! ReportID: %s", utilRes.ReportId)
	return utilRes, nil
}

// Внутренний метод ожидания готовности (Retry Logic)
func (w *WorkflowService) collectCodesWithRetry(orderId, gtin string, quantity int) (*models.CodesResponse, error) {
	maxAttempts := 15 // Ждем до 45 секунд (15 * 3 сек)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.GetLogger().Infof("WORKFLOW: Попытка %d: Проверка статуса подзаказа...", attempt)

		filters := map[string]string{
			"orderId": orderId,
			"gtin":    gtin,
		}

		subOrders, err := w.markingService.GetSubOrders(filters)
		if err != nil {
			logger.GetLogger().Infof("WORKFLOW: ❌ Ошибка GetSubOrders на попытке %d: %v", attempt, err)
			time.Sleep(3 * time.Second)
			continue
		}

		// 🔍 ДЕТАЛЬНОЕ ЛОГИРОВАНИЕ ОТВЕТА
		logger.GetLogger().Infof("WORKFLOW: ℹ️ Ответ от GetSubOrders: количество подзаказов = %d", len(subOrders.SubOrderInfos))

		if len(subOrders.SubOrderInfos) > 0 {
			info := subOrders.SubOrderInfos[0]
			logger.GetLogger().Infof("WORKFLOW: ℹ️ Статус буфера: %s, кодов в буфере: %d, требуется: %d", info.BufferStatus, info.LeftInBuffer, quantity)

			if info.BufferStatus == "ACTIVE" || info.BufferStatus == "READY" || info.BufferStatus == "EXHAUSTED" {
				logger.GetLogger().Infof("WORKFLOW: ✅ Буфер готов! Статус: %s", info.BufferStatus)
				if info.LeftInBuffer >= quantity {
					logger.GetLogger().Infof("WORKFLOW: ✅ В буфере достаточно кодов (%d). Продолжаем...", info.LeftInBuffer)
					return w.markingService.GetCodes(orderId, gtin, quantity, "")
				}
				if info.LeftInBuffer < quantity {
					return nil, fmt.Errorf("в буфере недостаточно кодов (доступно %d, запрошено %d)", info.LeftInBuffer, quantity)
				}

			}
			if info.BufferStatus == "REJECTED" {
				logger.GetLogger().Infof("WORKFLOW: ❌ Заказ отклонен: %s", info.RejectionReason)
				return nil, fmt.Errorf("заказ отклонен: %s", info.RejectionReason)
			}

			// Логируем неожиданный статус
			logger.GetLogger().Infof("WORKFLOW: ⏳ Статус буфера: %s (ожидаем READY/EXHAUSTED)", info.BufferStatus)
		} else {
			logger.GetLogger().Infof("WORKFLOW: ⏳ Подзаказы еще не найдены (попытка %d/15)", attempt)
		}

		time.Sleep(3 * time.Second)
	}
	logger.GetLogger().Infof("WORKFLOW: ❌ Превышено время ожидания готовности заказа после 15 попыток (~45 сек)")
	return nil, fmt.Errorf("превышено время ожидания готовности заказа")
}

// ReportAggregation - Подачи отчета об агрегации маркированных товаров
// Автоматически кодирует JSON в base64 и отправляет на API
func (w *WorkflowService) ReportAggregation(doc models.AggregationDocument) (*models.AggregationResponse, error) {
	logger.GetLogger().Infof("WORKFLOW: 📦 Подача отчета об агрегации: %d упаковок, BP=%d", len(doc.AggregationUnits), doc.BusinessPlaceId)

	// Вызываем MarkingService который сам все кодирует в base64
	result, err := w.markingService.ReportAggregation(doc)
	if err != nil {
		db.LogOperation("AGGREGATION", "N/A", "N/A", "ERROR", "Ошибка подачи агрегации: "+err.Error())
		return nil, fmt.Errorf("ошибка при подаче отчета об агрегации: %w", err)
	}

	db.LogOperation("AGGREGATION", "N/A", result.DocumentId, "SUCCESS", fmt.Sprintf("Агрегация зарегистрирована: %d упаковок", len(doc.AggregationUnits)))
	logger.GetLogger().Infof("WORKFLOW: ✅ Агрегация зарегистрирована! DocumentID: %s", result.DocumentId)

	return result, nil
}

