package out_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	var gitRepo string
	var bareGitRepo string
	var outSourceDir string

	var outCmd *exec.Cmd
	var output response

	BeforeEach(func() {
		var err error
		gitRepo, err = ioutil.TempDir("", "git-repo")
		Ω(err).ShouldNot(HaveOccurred())

		bareGitRepo, err = ioutil.TempDir("", "bare-git-repo")
		Ω(err).ShouldNot(HaveOccurred())

		outSourceDir, err = ioutil.TempDir("", "out-source-dir")
		Ω(err).ShouldNot(HaveOccurred())

		outCmd = exec.Command(outPath, outSourceDir)
	})

	JustBeforeEach(func() {
		jsonIn := fmt.Sprintf(`
			{
				"source": {
    			"uri": "%s",
    			"branch": "master",
          "pool": "lock-pool"
  			},
  			"params": {}
			}`, bareGitRepo)

		stdin, err := outCmd.StdinPipe()
		Ω(err).ShouldNot(HaveOccurred())

		session, err := gexec.Start(outCmd, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		_, err = stdin.Write([]byte(jsonIn))
		Ω(err).ShouldNot(HaveOccurred())
		stdin.Close()

		// account for roundtrip to s3
		Eventually(session, 5*time.Second).Should(gexec.Exit(0))

		err = json.Unmarshal(session.Out.Contents(), &output)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(outSourceDir)
		os.RemoveAll(bareGitRepo)
		os.RemoveAll(gitRepo)
	})

	Context("When acquiring a lock", func() {
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

        git add .
        git commit -m 'add some-lock'

				echo '{"some":"wrong-json"}' > lock-pool/unclaimed/some-other-lock

        git add .
        git commit -m 'add some-other-lock'
      `)

			gitSetup.Dir = gitRepo

			err := gitSetup.Run()
			Ω(err).ShouldNot(HaveOccurred())

			bareGitSetup := exec.Command("bash", "-e", "-c", fmt.Sprintf(`
        git clone %s --bare .
      `, gitRepo))
			bareGitSetup.Dir = bareGitRepo

			err = bareGitSetup.Run()
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("moves a lock to claimed", func() {
			gitSetup := exec.Command("git", "pull", bareGitRepo)
			gitSetup.Dir = gitRepo
			err := gitSetup.Run()
			Ω(err).ShouldNot(HaveOccurred())

			gitVersion := exec.Command("git", "rev-parse", "HEAD")
			gitVersion.Dir = gitRepo
			sha, err := gitVersion.Output()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(filepath.Join(gitRepo, "lock-pool", "claimed", "some-lock")).Should(BeARegularFile())

			Ω(output).Should(Equal(response{
				Version: version{
					Ref: strings.TrimSpace(string(sha)),
				},
				Metadata: []metadataPair{
					{Name: "lock_name", Value: "some-lock"},
					{Name: "pool_name", Value: "lock-pool"},
				},
			}))
		})
	})
})
