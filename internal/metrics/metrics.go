// Package metrics 提供 adortb-consent 服务的 Prometheus 指标。
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics 汇总 consent 服务的业务指标。
type Metrics struct {
	registry        *prometheus.Registry
	ConsentSaved    prometheus.Counter
	ConsentChecks   *prometheus.CounterVec
	GVLRefreshTotal prometheus.Counter
}

// New 创建并注册所有指标。
func New() *Metrics {
	reg := prometheus.NewRegistry()
	factory := promauto.With(reg)

	return &Metrics{
		registry: reg,
		ConsentSaved: factory.NewCounter(prometheus.CounterOpts{
			Name: "consent_saved_total",
			Help: "Total number of consent records saved",
		}),
		ConsentChecks: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "consent_check_total",
			Help: "Total number of consent checks",
		}, []string{"result"}), // result: allowed / denied
		GVLRefreshTotal: factory.NewCounter(prometheus.CounterOpts{
			Name: "gvl_refresh_total",
			Help: "Total number of GVL refreshes",
		}),
	}
}

// Handler 返回 Prometheus metrics HTTP handler。
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
