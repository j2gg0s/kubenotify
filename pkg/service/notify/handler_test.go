package notify

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
)

var excludes []*regexp.Regexp

func init() {
	excludes = append(excludes, regexp.MustCompile("metadata.*"))
	excludes = append(excludes, regexp.MustCompile("status.*"))
}

func ExampleDeployment() {
	f, err := os.Open("_data/deployments.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	handler := NewEventHandler(StdoutNotify(), excludes...)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		obj := &apps.Deployment{}
		if err := json.Unmarshal(scanner.Bytes(), &obj); err != nil {
			panic(err)
		}
		handler.OnAdd(obj)
	}

	// output:
}

func ExampleConfigMap() {
	f, err := os.Open("_data/configmaps.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	handler := NewEventHandler(StdoutNotify(), excludes...)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		obj := &core.ConfigMap{}
		if err := json.Unmarshal(scanner.Bytes(), &obj); err != nil {
			panic(err)
		}
		handler.OnAdd(obj)
	}

	// output:
}
