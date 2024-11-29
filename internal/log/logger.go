package log

import (
	"fmt"
	"os"

	"github.com/bombsimon/logrusr/v4"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"

	"github.com/openshift-assisted/ccx-exporter/internal/config"
)

var logger logr.Logger

func Init(conf config.Logs) error {
	loggerImpl := logrus.New()

	loggerImpl.SetLevel(logrus.Level(conf.Level + int(logrus.InfoLevel)))
	loggerImpl.SetOutput(os.Stdout)

	switch conf.Encoder {
	case config.EncoderTypeConsole:
		loggerImpl.SetFormatter(&logrus.TextFormatter{
			DisableColors: true,
		})
	case config.EncoderTypeJson:
		loggerImpl.SetFormatter(&logrus.JSONFormatter{})
	default:
		return fmt.Errorf("unexpected encoder value %v", conf.Encoder)
	}

	logger = logrusr.New(loggerImpl, logrusr.WithReportCaller())

	return nil
}

func Logger() logr.Logger {
	return logger
}
