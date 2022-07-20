package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

var branch = "refs/heads/master"

func main() {
	http.HandleFunc("/", index)
	http.ListenAndServe(":9999", nil)
}

// Having a struct with methods allows for
// Any number of instances to be restarted, not just one.
type restarter struct {
	user string
	path string
}

// Pull latest version.
// The script running the app will restart when it sees newer files.
func (r restarter) restart() {
	cmd := exec.Command("/usr/bin/bash")
	cmd.Stdin = strings.NewReader(`su ` + r.user + ` -c 'git pull'`)
	cmd.Dir = r.path
	out, err := cmd.CombinedOutput()
	log.Printf("script restart output: %q", string(out))
	if err != nil {
		fmt.Printf("error restarting site: %s", err)
		return
	}
	print(string(out))
}

// Kill running version so it can be restarted.
func (r restarter) killScript() {
	cmd := exec.Command("/usr/bin/bash")
	cmd.Stdin = strings.NewReader(`su ` + r.user + ` -c 'pkill -U ` + r.user + ` -f run.sh'`)
	cmd.Dir = r.path
	out, err := cmd.CombinedOutput()
	log.Printf("script kill output: %q", string(out))
	if err != nil {
		fmt.Printf("error killing script: %s", err)
		return
	}
}

func (r restarter) run(msg message) {
	if msg.Ref != branch {
		fmt.Printf("branch is %q; skipping", msg.Ref)
	}
	if msg.scriptUpdated() {
		r.killScript()
	}
	r.restart()
}

type message struct {
	Ref     string   `json:"ref"`
	Commits []commit `json:"commits"`
}

func (m message) scriptUpdated() bool {
	for _, c := range m.Commits {
		for _, mod := range c.Modified {
			if strings.HasSuffix(mod, ".sh") {
				log.Printf("script updated: %#v", m)
				return true
			}
		}
	}
	log.Printf("script not updated: %#v", m)
	return false
}

type commit struct {
	Modified []string `json:"modified"`
}

func getBody(body io.ReadCloser) ([]byte, error) {
	b, err := ioutil.ReadAll(body)
	if err == nil {
		body.Close()
	}
	return b, err
}

func getMessage(b []byte) (message, error) {
	var msg message
	err := json.Unmarshal(b, &msg)
	return msg, err
}

func index(w http.ResponseWriter, r *http.Request) {
	body, err := getBody(r.Body)
	if err != nil {
		log.Printf("error reading body: %s", err)
		return
	}

	msg, err := getMessage(body)
	if err != nil {
		log.Printf("error parsing body: %s", err)
		return
	}

	rDev := restarter{user: "site", path: "/home/site/ref_site"}
	rQA := restarter{user: "qa", path: "/home/qa/site"}

	go rDev.run(msg)
	go rQA.run(msg)

	fmt.Fprintf(w, "Hello, %s!", r.URL.Path[1:])
}
