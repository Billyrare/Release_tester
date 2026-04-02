// internal/models/product_card.go
package models

// ProductCard представляет карточку товара из реестра продукции
type ProductCard struct {
	ID              string            `json:"id"`
	GTIN            string            `json:"gtin"`
	ProductName     map[string]string `json:"productName"`
	INN             string            `json:"inn"`
	ProductGroup    ProductGroupInfo  `json:"productGroup"`
	ProductCategory CategoryInfo      `json:"productCategory"`
	Tnved           TnvedInfo         `json:"tnved"`
	PackageType     PackageTypeInfo   `json:"packageType"`
	Status          StatusInfo        `json:"status"`
	CreatedAt       string            `json:"createdAt"`
	UpdatedAt       string            `json:"updatedAt,omitempty"`
}

type ProductGroupInfo struct {
	Code string            `json:"code"`
	Name map[string]string `json:"name"`
}

type CategoryInfo struct {
	Code string            `json:"code"`
	Name map[string]string `json:"name"`
}

type TnvedInfo struct {
	Code string            `json:"code"`
	Name map[string]string `json:"name"`
}

type PackageTypeInfo struct {
	Code string            `json:"code"`
	Name map[string]string `json:"name"`
}

type StatusInfo struct {
	Code string            `json:"code"`
	Name map[string]string `json:"name"`
}

// ProductCardsResponse представляет ответ от API справочника товаров
type ProductCardsResponse struct {
	Cards []ProductCard `json:"cards"`
}
