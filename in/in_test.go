package in_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

type version struct {
	Ref string `json:"ref"`
}

type metadataPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type response struct {
	Version  version        `json:"version"`
	Metadata []metadataPair `json:"metadata"`
}

var _ = Describe("In", func() {
	var inDestination string
	var gitRepo string
	var destination string
	var inCmd *exec.Cmd
	var sha []byte
	var output response

	BeforeEach(func() {
		var err error
		inDestination, err = ioutil.TempDir("", "in-destination")
		gitRepo, err = ioutil.TempDir("", "git-repo")
		Ω(err).ShouldNot(HaveOccurred())

		destination = path.Join(inDestination, "in-dir")

		pwd, err := os.Getwd()
		Ω(err).ShouldNot(HaveOccurred())

		inPath := filepath.Join(pwd, "in")

		inCmd = exec.Command(inPath, inDestination)
	})

	JustBeforeEach(func() {
		stdin, err := inCmd.StdinPipe()
		Ω(err).ShouldNot(HaveOccurred())

		session, err := gexec.Start(inCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		jsonIn := fmt.Sprintf(`
			{
				"source": {
    			"uri": "%s",
    			"branch": "master"
  			},
  			"version": {
					"ref": "%s"
				}
			}`, gitRepo, string(sha))

		stdin.Write([]byte(jsonIn))
		stdin.Close()

		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))

		err = json.Unmarshal(session.Out.Contents(), &output)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(inDestination)
		os.RemoveAll(gitRepo)
	})

	Context("When a previous version is given", func() {
		BeforeEach(func() {

			gitSetup := exec.Command("bash", "-e", "-c", `
        git init

        mkdir -p lock-pool/unclaimed
        mkdir -p lock-pool/claimed

        touch lock-pool/unclaimed/.gitkeep
        touch lock-pool/claimed/.gitkeep

        touch lock-pool/unclaimed/some-lock
				touch lock-pool/unclaimed/some-other-lock

        echo '{"some":"json"}' > lock-pool/unclaimed/some-lock
				echo '{"some":"wrong-json"}' > lock-pool/unclaimed/some-other-lock

        git add .
        git commit -m 'test-git-setup'

				git mv  lock-pool/unclaimed/some-lock  lock-pool/claimed/some-lock
				git add .
				git commit -m 'claiming some-lock'
      `)

			gitSetup.Dir = gitRepo

			err := gitSetup.Run()
			Ω(err).ShouldNot(HaveOccurred())

			gitVersion := exec.Command("git", "rev-parse", "HEAD")

			gitVersion.Dir = gitRepo

			sha, err = gitVersion.Output()
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("outputs the metadata for the environment", func() {

			metaDataFile := filepath.Join(inDestination, "metadata")

			fileContents, err := ioutil.ReadFile(metaDataFile)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fileContents).Should(MatchJSON(`{"some":"json"}`))

			Ω(output).Should(Equal(response{
				Version: version{
					Ref: string(strings.TrimSpace(string(sha))),
				},
				Metadata: []metadataPair{
					{Name: "lock_name", Value: "some-lock"},
					{Name: "pool_name", Value: "lock-pool"},
				},
			}))
		})
	})
})
