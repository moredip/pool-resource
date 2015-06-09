package out_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestOut(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Out Suite")
}

var outPath string

var _ = BeforeSuite(func() {
	var err error

	outPath, err = gexec.Build("github.com/concourse/pool-resource/cmd/out")
	Ω(err).ShouldNot(HaveOccurred())
})
