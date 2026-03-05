package models

type OrderProduct struct {
	Gtin             string   `json:"gtin"`
	Quantity         int      `json:"quantity"`
	CisType          string   `json:"cisType"`
	SerialNumberType string   `json:"serialNumberType"` // operator, selfmeade
	SerialNumbers    []string `json:"serialNumbers,omitempty"`
}

// OrderRequest - структура основного ззапроса на заказ
type OrderRequest struct {
	ProductGroup      string         `json:"productGroup"`
	BusinessPlaceId   int            `json:"businessPlaceId,omitempty"`
	ReleaseMethodType string         `json:"releaseMethodType"`
	IsPaid            bool           `json:"isPaid"`
	Products          []OrderProduct `json:"products"`
}

// Ответ после запроса на OrderRequest - OrderResponse
type OrderResponse struct {
	OrderId string `json:"orderId"`
}
