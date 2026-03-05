// internal/models/marking_codes.go
package models

// PublicCodesRequest - структура для тела запроса к /public/api/cod/public/codes
type PublicCodesRequest struct {
	Codes []string `json:"codes"` // Массив кодов маркировки
}

// IssuerNameInfo - структура для имени эмитента на разных языках
type IssuerNameInfo struct {
	En string `json:"en"`
	Ru string `json:"ru"`
	Uz string `json:"uz"`
}

// IssuerShortInfo - структура для краткой информации об эмитенте КМ
type IssuerShortInfo struct {
	IssuerTin  string         `json:"issuerTin"`  // ИНН или ПИНФЛ эмитента
	IssuerName IssuerNameInfo `json:"issuerName"` // Наименование эмитента на разных языках
}

// AggregateProductGroup - структура для товарных групп вложенных КМ
type AggregateProductGroup struct {
	ProductGroupId int `json:"productGroupId"` // Код товарной группы
	UnitsNumber    int `json:"unitsNumber"`    // Количество вложенных потребительских КМ
}

// AggregateCategory - структура для товарных категорий вложенных КМ
type AggregateCategory struct {
	CategoryId  string `json:"categoryId"`  // Идентификатор категории товара
	UnitsNumber int    `json:"unitsNumber"` // Количество кодов в потребительской упаковке
}

// PublicCodeInfo - структура для одного элемента ответа от /public/api/cod/public/codes
type PublicCodeInfo struct {
	Code                   string                  `json:"code"`
	IsHadExtendedCode      bool                    `json:"isHadExtendedCode"` // Документация не содержит, но есть в примере
	PackageType            string                  `json:"packageType"`
	Status                 string                  `json:"status"`
	ExtendedStatus         string                  `json:"extendedStatus"`
	IssuerShortInfo        IssuerShortInfo         `json:"issuerShortInfo"`
	Template               string                  `json:"template"`
	Gtin                   string                  `json:"gtin"`
	ProductId              string                  `json:"productId"`
	ProductGroupId         int                     `json:"productGroupId"`
	CategoryId             string                  `json:"categoryId"`
	EmissionDate           string                  `json:"emissionDate"`
	IssueDate              string                  `json:"issueDate"`
	ProductionDate         string                  `json:"productionDate"`
	ExpirationDate         string                  `json:"expirationDate"`
	ProductSeries          string                  `json:"productSeries"`
	MixedProductGroups     bool                    `json:"mixedProductGroups"`
	MixedCategories        bool                    `json:"mixedCategories"`
	AggregateProductGroups []AggregateProductGroup `json:"aggregateProductGroups"` // Массив
	AggregateCategories    []AggregateCategory     `json:"aggregateCategories"`    // Массив
}
