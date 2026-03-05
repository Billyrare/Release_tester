// internal/models/utilisation.go
package models

// UtilisationRequest - тело запроса на нанесение КМ
type UtilisationRequest struct {
	Sntins              []string `json:"sntins"`              // Массив полных кодов маркировки
	BusinessPlaceId     int      `json:"businessPlaceId"`     // ID места деятельности
	ManufacturerCountry string   `json:"manufacturerCountry"` // Код страны (например, "UZ")
	ProductionOrderId   string   `json:"productionOrderId,omitempty"`
	ReleaseType         string   `json:"releaseType"`    // PRODUCTION или IMPORT
	ProductionDate      string   `json:"productionDate"` // ISO 8601
	ExpirationDate      string   `json:"expirationDate"` // ISO 8601
	SeriesNumber        string   `json:"seriesNumber,omitempty"`
}

// UtilisationResponse - ответ системы
type UtilisationResponse struct {
	ReportId string `json:"reportId"`
}
