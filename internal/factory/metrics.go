package factory

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/openshift-assisted/ccx-exporter/internal/config"
)

func CreatePrometheusServer(conf config.Metrics, gatherer prometheus.Gatherer) *http.Server {
	ret := &http.Server{Addr: fmt.Sprintf(":%v", conf.Port)}
	ret.SetKeepAlivesEnabled(true)
	ret.IdleTimeout = 5 * time.Second

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	ret.Handler = router

	return ret
}
