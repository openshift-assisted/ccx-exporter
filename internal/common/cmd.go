package common

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/dustin/go-humanize"
	"go.uber.org/automaxprocs/maxprocs"

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
		// logger.Error(coverage.WriteCountersDir("/coverdata"), "writing counter dir")
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
