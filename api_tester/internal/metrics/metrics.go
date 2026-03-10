// internal/metrics/metrics.go
// Централизованное определение всех Prometheus-метрик приложения
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// ===== HTTP =====
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Общее количество HTTP-запросов",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Время обработки HTTP-запроса",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"method", "path"},
	)

	HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Количество HTTP-запросов в обработке прямо сейчас",
		},
	)

	// ===== WORKFLOW =====
	WorkflowExecutionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workflow_executions_total",
			Help: "Количество запусков workflow",
		},
		[]string{"status", "product_group"},
	)

	WorkflowDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "workflow_duration_seconds",
			Help:    "Полное время выполнения workflow",
			Buckets: []float64{5, 10, 20, 30, 45, 60, 90, 120, 180, 300},
		},
		[]string{"product_group"},
	)

	WorkflowCodesGenerated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workflow_codes_generated_total",
			Help: "Количество сгенерированных кодов маркировки",
		},
		[]string{"product_group"},
	)

	WorkflowRetryAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "workflow_retry_attempts_total",
			Help: "Количество retry-попыток при ожидании кодов",
		},
		[]string{"product_group"},
	)

	// ===== UTILISATION =====
	UtilisationReportsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "utilisation_reports_total",
			Help: "Количество отчётов о нанесении маркировки",
		},
		[]string{"status", "product_group"},
	)

	UtilisationCodesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "utilisation_codes_total",
			Help: "Суммарное количество кодов в отчётах о нанесении",
		},
		[]string{"product_group"},
	)

	// ===== AGGREGATION =====
	AggregationReportsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aggregation_reports_total",
			Help: "Количество отчётов об агрегации",
		},
		[]string{"status"},
	)

	// ===== ВНЕШНИЙ API (ASL) =====
	AslAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "asl_api_requests_total",
			Help: "Количество запросов к внешнему API ASL Belgisi",
		},
		[]string{"endpoint", "status"},
	)

	AslAPIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "asl_api_request_duration_seconds",
			Help:    "Время ответа внешнего API ASL Belgisi",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"endpoint"},
	)

	AslAPIRetryAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "asl_api_retry_attempts_total",
			Help: "Количество повторных попыток запросов к ASL API",
		},
		[]string{"endpoint"},
	)

	// ===== CODES =====
	CodesSavedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "codes_saved_total",
			Help: "Количество сохранённых кодов в файлы",
		},
		[]string{"product_group"},
	)

	// ===== DURATION =====
	UtilisationRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "utilisation_request_duration_seconds",
			Help:    "Время выполнения запроса нанесения",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
		[]string{"product_group"},
	)

	AggregationDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "aggregation_duration_seconds",
			Help:    "Время выполнения агрегации",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
		},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		HTTPRequestsInFlight,
		WorkflowExecutionsTotal,
		WorkflowDuration,
		WorkflowCodesGenerated,
		WorkflowRetryAttempts,
		UtilisationReportsTotal,
		UtilisationCodesTotal,
		AggregationReportsTotal,
		AslAPIRequestsTotal,
		AslAPIRequestDuration,
		AslAPIRetryAttempts,
		CodesSavedTotal,
		UtilisationRequestDuration,
		AggregationDuration,
	)
}
