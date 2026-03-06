package models

type WorkflowResponse struct {
	ReportId            string   `json:"report_id"`
	CodesForAggregation []string `json:"codes_for_aggregation"`
}
