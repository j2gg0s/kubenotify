package main

import (
	"bufio"
	"encoding/json"
	"os"

	apps "k8s.io/api/apps/v1"
)

func ExampleDeployment() {
	if f, err := os.Open("_data/deployments.json"); err != nil {
		panic(err)
	} else {
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			deployment := &apps.Deployment{}
			if err := json.Unmarshal(scanner.Bytes(), deployment); err != nil {
				panic(err)
			}
			if err := Add(deployment, &DeploymentResource{}); err != nil {
				panic(err)
			}
		}
	}

	// output: error
}
