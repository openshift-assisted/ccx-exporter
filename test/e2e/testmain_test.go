package e2e_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var kubeconfig string // path to kubeconfig file

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", defaultKubeConfigFilePath(), "path to the kubeconfig file")
}

func defaultKubeConfigFilePath() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	// move up twice to go back to the root dir
	rootDir := wd
	if filepath.Base(wd) == "e2e" {
		rootDir = filepath.Dir(filepath.Dir(wd))
	}

	ret := filepath.Join(rootDir, "build", "kubeconfig")
	_, err = os.Stat(ret)
	if err != nil {
		return ""
	}

	return ret
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// Go Test
func TestCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common test suite")
}
