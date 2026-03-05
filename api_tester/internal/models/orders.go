package models

type OrderInfo struct {
	OrderId           string `json:"orderId"`
	ProductGroup      string `json:"productGroup"`
	OrderStatus       string `json:"orderStatus"`
	ReleaseMethodType string `json:"releaseMethodType"`
	PoNumber          string `json:"poNumber"`
	CreateDate        string `json:"createDate"`
	RejectionReason   string `json:"rejectionReason,omitempty"`
}

type OrderListResponse struct {
	OrderInfos []OrderInfo `json:"orderInfos"`
}
