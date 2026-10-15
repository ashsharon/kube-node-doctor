// Command kube-node-doctor prints a quick health report for a Kubernetes
// cluster: which nodes are Ready and pressure-free, and which pods are
// crash-looping, stuck Pending, or restarting too often.
//
// It is a thin, dependency-free wrapper around `kubectl get nodes/pods -o
// json` — no client-go, no cluster-side install, works with whatever
// kubeconfig context is already active.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ashsharon/kube-node-doctor/internal/k8s"
	"github.com/ashsharon/kube-node-doctor/internal/report"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("kube-node-doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)

	namespace := fs.String("namespace", "", "only scan pods in this namespace (default: all namespaces)")
	kubeContext := fs.String("context", "", "kubeconfig context to use (default: current context)")
	restartThreshold := fs.Int("restart-threshold", 5, "flag a pod once any container has restarted this many times")
	timeout := fs.Duration("timeout", 15*time.Second, "timeout for each kubectl call")
	asJSON := fs.Bool("json", false, "print the report as JSON instead of a table")
	failOnIssue := fs.Bool("fail-on-issue", false, "exit with status 1 if any node or pod is unhealthy (handy in CI)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	runner := k8s.KubectlRunner{Context: *kubeContext, Timeout: *timeout}
	r, err := report.Collect(runner, report.Options{
		Namespace:        *namespace,
		RestartThreshold: *restartThreshold,
	})
	if err != nil {
		fmt.Fprintf(stderr, "kube-node-doctor: %v\n", err)
		return 1
	}

	if *asJSON {
		if err := report.RenderJSON(stdout, r); err != nil {
			fmt.Fprintf(stderr, "kube-node-doctor: %v\n", err)
			return 1
		}
	} else {
		report.RenderTable(stdout, r)
	}

	if *failOnIssue && (r.HealthyNodes() != len(r.Nodes) || len(r.ProblemPods) > 0) {
		return 1
	}
	return 0
}
