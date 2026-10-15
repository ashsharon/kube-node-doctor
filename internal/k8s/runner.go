// Package k8s provides a small, mockable wrapper around the kubectl CLI.
//
// kube-node-doctor deliberately shells out to kubectl instead of vendoring
// client-go: it keeps the binary tiny, it always speaks whatever API
// version the cluster's kubectl understands, and it means the tool works
// anywhere kubectl already works (including behind a kubeconfig context
// switch) with zero extra auth plumbing.
package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Runner executes a kubectl subcommand and returns its stdout. It is the
// single seam the rest of the program depends on, which is what makes the
// report logic testable without a real cluster (see fake_runner_test.go).
type Runner interface {
	Run(args ...string) ([]byte, error)
}

// KubectlRunner is the production Runner: it shells out to the real
// kubectl binary on PATH.
type KubectlRunner struct {
	// Context, if set, is passed as --context to every invocation.
	Context string
	// Timeout bounds a single kubectl call. Zero means no timeout.
	Timeout time.Duration
}

func (r KubectlRunner) Run(args ...string) ([]byte, error) {
	ctx := context.Background()
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	if r.Context != "" {
		args = append([]string{"--context", r.Context}, args...)
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kubectl %v: %w (stderr: %s)", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
