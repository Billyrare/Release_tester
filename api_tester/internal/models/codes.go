package models

// CodesResponse - ответ со списком выгруженных кодов
type CodesResponse struct {
	PackId string   `json:"packId"`
	Codes  []string `json:"codes"`
}
