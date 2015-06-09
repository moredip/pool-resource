package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Source struct {
	URI        string `json:"uri"`
	Branch     string `json:"branch"`
	PrivateKey string `json:"private_key"`
	Pool       string `json:"pool"`
}

type Version struct {
	Ref string `json:"ref"`
}

type OutParams struct {
	Release bool `json:"release"`
}

type OutRequest struct {
	Source Source    `json:"source"`
	Params OutParams `json:"params"`
}

type OutResponse struct {
	Version  Version        `json:"version"`
	Metadata []MetadataPair `json:"metadata"`
}

type MetadataPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func main() {
	if len(os.Args) < 2 {
		println("usage: " + os.Args[0] + " <source>")
		os.Exit(1)
	}

	var request OutRequest
	err := json.NewDecoder(os.Stdin).Decode(&request)
	if err != nil {
		fatal("reading request", err)
	}

	pools := Pools{}

	lock, version, err := pools.AcquireLock(request.Source.Pool)
	if err != nil {
		log.Fatalln(err)
	}

	err = json.NewEncoder(os.Stdout).Encode(OutResponse{
		Version: version,
		Metadata: []MetadataPair{
			{Name: "lock_name", Value: lock},
		},
	})
	if err != nil {
		fatal("encoding output", err)
	}
}

func fatal(doing string, err error) {
	println("error " + doing + ": " + err.Error())
	os.Exit(1)
}

type Pools struct {
	Source Source

	dir string
}

func (p *Pools) AcquireLock(pool string) (string, Version, error) {
	err := p.setup()
	if err != nil {
		return "", Version{}, err
	}

	var (
		lock                string
		broadcastSuccessful bool
	)

	for broadcastSuccessful {
		lock, err = p.grabAvailableLock(pool)

		err = p.broadcastLocked()
		if err == nil {
			broadcastSuccessful = true
		} else {
			err = p.resetLock()
			if err != nil {
				// idk
				log.Fatalln(err)
			}
		}

		time.Sleep(30 * time.Second)
	}

	return lock, Version{}, nil
}

func (p *Pools) ReleaseLockFromVersion(version Version) (Version, error) {
	return Version{}, nil
}

func (p *Pools) resetLock() error {
	err := p.git("reset", "--hard", "origin/"+p.Source.Branch)
	if err != nil {
		return err
	}

	err = p.git("branch", "-f", p.Source.Branch)
	if err != nil {
		return err
	}

	return nil
}

func (p *Pools) setup() error {
	var err error

	p.dir, err = ioutil.TempDir("", "pool-resource")
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "clone", p.Source.URI, p.dir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	err = p.git("config", "user.name", "CI Pool Resource")
	if err != nil {
		return err
	}

	err = p.git("config", "user.email", "ci-pool@localhost")
	if err != nil {
		return err
	}

	return nil
}

func (p *Pools) grabAvailableLock(pool string) (string, error) {
	files, err := ioutil.ReadDir(filepath.Join(p.dir, pool, "unclaimed"))
	if err != nil {
		return "", err
	}

	rand.Seed(time.Now().Unix())
	index := rand.Int() % len(files)
	name := filepath.Base(files[index].Name())

	err = p.git("mv", filepath.Join(pool, "unclaimed", name), filepath.Join(pool, "claimed", name))
	if err != nil {
		return "", err
	}

	err = p.git("commit", "-am", fmt.Sprintf("claiming: %s", name))
	if err != nil {
		return "", err
	}

	return name, nil
}

func (p *Pools) broadcastLocked() error {
	return p.git("push", "origin", p.Source.Branch)
}

func (p *Pools) git(args ...string) error {
	arguments := append([]string{"-C", p.dir}, args...)
	cmd := exec.Command("git", arguments...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

/*
Clone git respo
find available lock
  - if not available, loop until available with sleep TODO
  - if available, git move the file to claimed
Commit lock
push lock to repo
  - if push fails
  - give up and start over - state of locks has changed
  - attempt to re-aqcuire a lock
*/
