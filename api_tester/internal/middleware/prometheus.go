// internal/middleware/prometheus.go
// Gin middleware для автоматического сбора HTTP-метрик
package middleware

import (
	"api_tester/internal/metrics"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// PrometheusMiddleware собирает метрики для каждого HTTP-запроса
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath() // /v1/workflow/execute (нормализованный путь с :param)
		if path == "" {
			path = c.Request.URL.Path // fallback
		}
		method := c.Request.Method

		metrics.HTTPRequestsInFlight.Inc()
		defer metrics.HTTPRequestsInFlight.Dec()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}
