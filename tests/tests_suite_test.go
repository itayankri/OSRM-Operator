package tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestFunctional(t *testing.T) {
	var reporters []Reporter
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Functional test suite", reporters)
}
