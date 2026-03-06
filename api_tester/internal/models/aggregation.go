// internal/models/aggregation.go
package models

// AggregationUnit - описание одной коробки/паллеты
type AggregationUnit struct {
	AggregationItemsCount   int      `json:"aggregationItemsCount"`   // Сколько реально внутри
	AggregationUnitCapacity int      `json:"aggregationUnitCapacity"` // Сколько влезет максимум
	Codes                   []string `json:"codes"`                   // Коды бутылок/пачек внутри
	ShouldBeUnbundled       bool     `json:"shouldBeUnbundled"`       // Распаковать старое, если было?
	UnitSerialNumber        string   `json:"unitSerialNumber"`        // Код самой коробки (SSCC)
}

// AggregationDocument - структура самого отчета (то, что станет base64)
type AggregationDocument struct {
	AggregationUnits  []AggregationUnit `json:"aggregationUnits"`
	BusinessPlaceId   int               `json:"businessPlaceId"`
	DocumentDate      string            `json:"documentDate"` // ISO 8601
	ProductionOrderId string            `json:"productionOrderId,omitempty"`
}

// AggregationRequest - то, что мы шлем на сервер
type AggregationRequest struct {
	DocumentBody string `json:"documentBody"` // Сюда мы положим Base64 от AggregationDocument
	Signature    string `json:"signature,omitempty"`
}

// AggregationResponse - ответ от сервера
type AggregationResponse struct {
	DocumentId string `json:"documentId"`
}
