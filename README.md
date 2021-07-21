# kubenotify

[![Docs](https://godoc.org/github.com/j2gg0s/kubenotify?status.svg)](https://pkg.go.dev/github.com/j2gg0s/kubenotify)
[![Go Report Card](https://goreportcard.com/badge/github.com/j2gg0s/kubenotify)](https://goreportcard.com/report/github.com/j2gg0s/kubenotify)

`kubenotify` is a Kubernetes watcher that publishes notification to available webhooks.
Run it in your k8s cluster, you can receive workload change notifications through webhooks.

## Install

```
$ get get github.com/j2gg0s/kubenotify
```

## Usage

```
kubenotify -h
subscribe kubernetes workload change event, support Deployment, StatefulSet and DaemonSet

Usage:
  kubenotify [flags]

Flags:
      --debug                  enable debug log
      --disable-revision       disable revision (default true)
      --excludes strings       excludes resource field when diff (default [metadata\.[acdfgmors].*,status\..*])
  -h, --help                   help for kubenotify
      --ignore-before string   ignore create before when start (default "1m")
      --includes strings       only include resource field when diff
      --namespaces strings     watch resource under these namepsace, default all
      --outof-cluster          use outof cluster config directly
      --resources strings      watch only these resource, default all, support Deployment, StatefulSet, DaemonSet
      --resync string          duration to resync resource (default "1m")
      --webhooks strings       webhook to notify

```
