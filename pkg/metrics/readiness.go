package metrics

import "github.com/prometheus/client_golang/prometheus"

type ReadinessGauge struct {
	gauge *prometheus.GaugeVec
}

func NewReadinessGauge(reg *Registry) *ReadinessGauge {
	if reg == nil {
		return nil
	}

	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "invenlore_readiness",
			Help: "Readiness status of service components (0/1).",
		},
		[]string{"component"},
	)

	reg.Registerer.MustRegister(gauge)

	return &ReadinessGauge{gauge: gauge}
}

func (r *ReadinessGauge) Set(component string, ready bool) {
	if r == nil || r.gauge == nil {
		return
	}

	if ready {
		r.gauge.WithLabelValues(component).Set(1)
		return
	}

	r.gauge.WithLabelValues(component).Set(0)
}
