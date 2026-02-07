package metrics

import (
	"net/http"

	"github.com/invenlore/core/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	Registry   *prometheus.Registry
	Registerer prometheus.Registerer
	Labels     prometheus.Labels
	Service    string
	Env        string
	Version    string
}

func NewRegistry(service string, env config.AppEnv, version string) *Registry {
	labels := prometheus.Labels{
		"service": service,
		"env":     env.String(),
		"version": version,
	}

	reg := prometheus.NewRegistry()
	registerer := prometheus.WrapRegistererWith(labels, reg)

	serviceInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "service_info",
		Help: "Static service metadata (service, env, version) with value 1.",
	})

	registerer.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		serviceInfo,
	)

	serviceInfo.Set(1)

	return &Registry{
		Registry:   reg,
		Registerer: registerer,
		Labels:     labels,
		Service:    service,
		Env:        env.String(),
		Version:    version,
	}
}

func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.Registry, promhttp.HandlerOpts{})
}
