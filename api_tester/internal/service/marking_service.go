// internal/service/marking_service.go
package service

import (
	"api_tester/internal/db"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"api_tester/config"
	"api_tester/internal/models"
)

// MarkingService определяет интерфейс для работы с API маркировки
type MarkingService interface {
	GetPublicCodesInfo(codes []string) ([]models.PublicCodeInfo, error)
	// --- НОВЫЙ МЕТОД ---
	CreateOrder(order models.OrderRequest) (*models.OrderResponse, error)

	GetOrders(filters map[string]string) (*models.OrderListResponse, error)

	GetCodes(orderId, gtin string, quantity int, lastPackId string) (*models.CodesResponse, error)

	GetSubOrders(filters map[string]string) (*models.SubOrderListResponse, error)

	ReportUtilisation(productGroup string, data models.UtilisationRequest) (*models.UtilisationResponse, error)

	ReportAggregation(data models.AggregationDocument) (*models.AggregationResponse, error)
}

// markingService implements MarkingService
type markingService struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewMarkingService создает новый экземпляр MarkingService
func NewMarkingService(cfg *config.Config) MarkingService {
	return &markingService{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second, // Немного увеличим таймаут для заказов
		},
	}
}

// GetPublicCodesInfo - получение информации о кодах (уже проверено)
func (s *markingService) GetPublicCodesInfo(codes []string) ([]models.PublicCodeInfo, error) {
	requestBody := models.PublicCodesRequest{
		Codes: codes,
	}

	var jsonBody bytes.Buffer
	encoder := json.NewEncoder(&jsonBody)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(requestBody); err != nil {
		return nil, fmt.Errorf("ошибка кодирования: %w", err)
	}

	requestURL := fmt.Sprintf("%s/public/api/cod/public/codes", s.cfg.AslApiURL)
	req, err := http.NewRequest("POST", requestURL, &jsonBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.AslApiToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s, body: %s", resp.Status, string(bodyBytes))
	}

	var responseData []models.PublicCodeInfo
	if err := json.Unmarshal(bodyBytes, &responseData); err != nil {
		return nil, err
	}

	return responseData, nil
}

// --- НОВАЯ РЕАЛИЗАЦИЯ: CreateOrder ---
func (s *markingService) CreateOrder(order models.OrderRequest) (*models.OrderResponse, error) {

	var jsonBody bytes.Buffer
	encoder := json.NewEncoder(&jsonBody)
	encoder.SetEscapeHTML(false) // Чтобы спецсимволы в GTIN не кодировались
	if err := encoder.Encode(order); err != nil {
		log.Printf("ERROR: Ошибка кодирования заказа: %v", err)
		return nil, fmt.Errorf("ошибка кодирования заказа: %w", err)
	}

	// URL из документации: https://{server}/api/orders
	requestURL := fmt.Sprintf("%s/api/orders", s.cfg.AslApiURL)

	req, err := http.NewRequest("POST", requestURL, &jsonBody)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать запрос: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.AslApiToken))

	log.Printf("INFO: Отправка запроса на создание заказа в ASL BELGISI: %s", requestURL)
	// Опционально: лог тела запроса для отладки заказа
	// log.Printf("DEBUG: Тело заказа: %s", jsonBody.String())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("ERROR: Ошибка HTTP-запроса при создании заказа: %v", err)
		return nil, fmt.Errorf("ошибка HTTP-запроса: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать ответ: %w", err)
	}

	log.Printf("INFO: Ответ от API заказов. Статус: %s", resp.Status)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("ERROR: API заказа вернуло ошибку: %s. Тело: %s", resp.Status, string(bodyBytes))
		return nil, fmt.Errorf("API вернуло ошибку: %s, Тело: %s", resp.Status, string(bodyBytes))
	}

	var orderResponse models.OrderResponse
	if err := json.Unmarshal(bodyBytes, &orderResponse); err != nil {
		log.Printf("ERROR: Ошибка десериализации ответа заказа: %v", err)
		return nil, fmt.Errorf("не удалось разобрать ответ: %w", err)
	}

	db.LogOperation("ORDER", order.ProductGroup, orderResponse.OrderId, "SUCCESS", fmt.Sprintf("Количество: %d", len(order.Products)))

	log.Printf("INFO: Заказ успешно создан. OrderID: %s", orderResponse.OrderId)
	return &orderResponse, nil

}

// GetOrders получает список заказов с фильтрами
func (s *markingService) GetOrders(filters map[string]string) (*models.OrderListResponse, error) {
	// 1. Формируем URL с Query-параметрами
	baseURL := fmt.Sprintf("%s/api/orders", s.cfg.AslApiURL)
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга URL: %w", err)
	}

	// Добавляем фильтры из мапы в URL
	q := u.Query()
	for key, value := range filters {
		if value != "" {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()
	finalURL := u.String()

	// 2. Создаем GET запрос (тело nil, так как это GET)
	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	// 3. Заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.AslApiToken))

	log.Printf("INFO: Запрос списка заказов: %s", finalURL)

	// 4. Выполнение запроса
	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("ERROR: Ошибка HTTP-запроса: %v", err)
		return nil, fmt.Errorf("ошибка HTTP-запроса: %w", err)
	}
	defer resp.Body.Close()

	// 5. Чтение ответа
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать ответ: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (Status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// 6. Десериализация JSON в нашу структуру OrderListResponse
	var orderList models.OrderListResponse
	if err := json.Unmarshal(bodyBytes, &orderList); err != nil {
		log.Printf("ERROR: Ошибка разбора JSON: %v", err)
		return nil, fmt.Errorf("ошибка разбора ответа: %w", err)
	}

	log.Printf("INFO: Успешно получено %d заказов", len(orderList.OrderInfos))
	return &orderList, nil
}

// GetCodes - выгрузка кодов маркировки из подзаказа
func (s *markingService) GetCodes(orderId, gtin string, quantity int, lastPackId string) (*models.CodesResponse, error) {
	baseURL := fmt.Sprintf("%s/api/codes", s.cfg.AslApiURL)
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Add("orderId", orderId)
	q.Add("gtin", gtin)
	q.Add("quantity", fmt.Sprintf("%d", quantity))
	if lastPackId != "" && lastPackId != "0" {
		q.Add("lastPackId", lastPackId)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.AslApiToken))

	log.Printf("INFO: Запрос на выгрузку КМ: %s", u.String())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка выгрузки (Status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var codesRes models.CodesResponse
	if err := json.Unmarshal(bodyBytes, &codesRes); err != nil {
		return nil, err
	}

	log.Printf("INFO: Успешно выгружено %d кодов. PackID: %s", len(codesRes.Codes), codesRes.PackId)
	return &codesRes, nil
}

// GetSubOrders - получение информации о подзаказах (остатки кодов)
func (s *markingService) GetSubOrders(filters map[string]string) (*models.SubOrderListResponse, error) {
	baseURL := fmt.Sprintf("%s/api/orders/sub-orders", s.cfg.AslApiURL)
	u, _ := url.Parse(baseURL)
	q := u.Query()
	for k, v := range filters {
		if v != "" {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()

	finalURL := u.String()
	log.Printf("DEBUG GetSubOrders: URL запроса: %s", finalURL)

	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		log.Printf("ERROR GetSubOrders: Ошибка создания запроса: %v", err)
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.AslApiToken))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("ERROR GetSubOrders: Ошибка HTTP-запроса: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("DEBUG GetSubOrders: HTTP статус: %s", resp.Status)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR GetSubOrders: Ошибка чтения тела ответа: %v", err)
		return nil, err
	}

	log.Printf("DEBUG GetSubOrders: Тело ответа: %s", string(body))

	var res models.SubOrderListResponse
	if err := json.Unmarshal(body, &res); err != nil {
		log.Printf("ERROR GetSubOrders: Ошибка разбора JSON: %v", err)
		log.Printf("ERROR GetSubOrders: Raw body: %s", string(body))
		return nil, err
	}

	log.Printf("DEBUG GetSubOrders: Успешно получено %d подзаказов", len(res.SubOrderInfos))
	return &res, nil
}

func (s *markingService) ReportUtilisation(productGroup string, data models.UtilisationRequest) (*models.UtilisationResponse, error) {
	// Corrected: Create a new UtilisationRequest with the correctly formatted dates
	correctData := models.UtilisationRequest{
		Sntins:              data.Sntins,
		BusinessPlaceId:     data.BusinessPlaceId,
		ManufacturerCountry: data.ManufacturerCountry,
		ReleaseType:         data.ReleaseType,
		ProductionDate:      data.ProductionDate, // Assuming this is already in "YYYY-MM-DD" format
		ExpirationDate:      data.ExpirationDate, // Assuming this is already in "YYYY-MM-DD" format
	}

	var jsonBody bytes.Buffer
	encoder := json.NewEncoder(&jsonBody)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(correctData); err != nil {
		return nil, fmt.Errorf("ошибка кодирования: %w", err)
	}

	// URL: https://{server}/api/utilisation?productGroup=...
	baseURL := fmt.Sprintf("%s/api/utilisation", s.cfg.AslApiURL)
	u, _ := url.Parse(baseURL)
	q := u.Query()
	q.Add("productGroup", productGroup)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("POST", u.String(), &jsonBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.AslApiToken))

	log.Printf("INFO: Отправка отчета о нанесении (%d кодов) для группы %s", len(data.Sntins), productGroup)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("ошибка API (Status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res models.UtilisationResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, err
	}

	db.LogOperation("UTILISATION", productGroup, res.ReportId, "SUCCESS", fmt.Sprintf("Нанесено %d кодов", len(data.Sntins)))

	log.Printf("INFO: Отчет о нанесении принят. ReportID: %s", res.ReportId)
	return &res, nil
}

// Добавь "encoding/base64" в импорты

func (s *markingService) ReportAggregation(doc models.AggregationDocument) (*models.AggregationResponse, error) {
	// 1. Кодируем внутренний документ в JSON
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга документа: %w", err)
	}

	// 2. Превращаем JSON в Base64
	encodedDoc := base64.StdEncoding.EncodeToString(docBytes)

	// 3. Формируем финальное тело запроса
	finalRequest := models.AggregationRequest{
		DocumentBody: encodedDoc,
	}

	var jsonBody bytes.Buffer
	json.NewEncoder(&jsonBody).Encode(finalRequest)

	// URL: https://{server}/public/api/v1/doc/aggregation
	url := fmt.Sprintf("%s/public/api/v1/doc/aggregation", s.cfg.AslApiURL)

	req, err := http.NewRequest("POST", url, &jsonBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.AslApiToken))

	log.Printf("INFO: Отправка агрегации для %d упаковок", len(doc.AggregationUnits))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("ошибка агрегации (Status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res models.AggregationResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, err
	}

	db.LogOperation("AGGREGATION", "N/A", res.DocumentId, "SUCCESS", "Агрегация выполнена успешно")

	return &res, nil
}
