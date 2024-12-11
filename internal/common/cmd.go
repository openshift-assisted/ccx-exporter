package common

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/dustin/go-humanize"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/automaxprocs/maxprocs"

	"github.com/openshift-assisted/ccx-exporter/internal/config"
	"github.com/openshift-assisted/ccx-exporter/internal/log"
)

const (
	// default ratio from the memlimit pkg
	memLimitRatio = 0.9
)

func SetupSignalHandler(ctx context.Context) context.Context {
	ret, cancel := context.WithCancel(ctx)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		logger := log.Logger()

		<-c
		logger.V(1).Info("Signal received to stop")
		cancel()

		<-c
		logger.V(0).Info("Re-receiving stop signal, exit directly")
		os.Exit(1)
	}()

	return ret
}

func SetMaxProcs() error {
	logger := log.Logger()

	// maxprocs uses a logger with parameters: $template, $arg1, $arg2, ... whereas logr has the same signature but different meaning: $msg, $key1, $value1, $key2, $value2, ...
	_, err := maxprocs.Set(maxprocs.Logger(func(msg string, args ...interface{}) {
		logger.Info(fmt.Sprintf(msg, args...))
	}))
	if err != nil {
		return fmt.Errorf("failed to set max procs: %w", err)
	}

	return nil
}

func SetMemLimit() error {
	logger := log.Logger()

	limit, err := memlimit.SetGoMemLimit(memLimitRatio)
	if err != nil {
		return fmt.Errorf("failed to set go mem limit: %w", err)
	}

	logger.V(1).Info("Go memlimit configured", "ratio", memLimitRatio, "limit", humanize.IBytes(uint64(limit)))

	return nil
}

type CloseFunc func(context.Context) error

func StartPrometheusServer(conf config.Metrics, gatherer prometheus.Gatherer) CloseFunc {
	logger := log.Logger()

	logger.V(4).Info("Starting prometheus server")

	srv := &http.Server{Addr: fmt.Sprintf(":%v", conf.Port)}
	srv.SetKeepAlivesEnabled(true)
	srv.IdleTimeout = 5 * time.Second

	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	srv.Handler = router

	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Error(err, "Prometheus server stopped")
		}
	}()

	return func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}
}
