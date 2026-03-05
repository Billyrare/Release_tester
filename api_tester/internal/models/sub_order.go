// internal/models/sub_order.go
package models

type SubOrderInfo struct {
	ParentOrderId   string `json:"parentOrderId"`
	Gtin            string `json:"gtin"`
	BufferStatus    string `json:"bufferStatus"`
	CisType         string `json:"cisType"`
	AvailableCodes  int    `json:"availableCodes"`
	LeftInBuffer    int    `json:"leftInBuffer"`
	TotalPassed     int    `json:"totalPassed"`
	LastPackId      string `json:"lastPackId,omitempty"`
	CreateDate      string `json:"createDate"`
	RejectionReason string `json:"rejectionReason,omitempty"`
}

type SubOrderListResponse struct {
	SubOrderInfos []SubOrderInfo `json:"subOrderInfos"`
}
