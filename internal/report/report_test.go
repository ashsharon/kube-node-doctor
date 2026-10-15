package report

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// fakeRunner implements k8s.Runner over canned fixture files, keyed by
// whether the call was "get nodes" or "get pods". This is the whole reason
// Runner is an interface: the report logic never needs a real cluster to
// be tested.
type fakeRunner struct {
	nodesFile string
	podsFile  string
}

func (f fakeRunner) Run(args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	switch {
	case strings.HasPrefix(joined, "get nodes"):
		return os.ReadFile(f.nodesFile)
	case strings.HasPrefix(joined, "get pods"):
		return os.ReadFile(f.podsFile)
	default:
		return nil, fmt.Errorf("fakeRunner: unexpected call %q", joined)
	}
}

func TestCollect_MixedHealth(t *testing.T) {
	runner := fakeRunner{
		nodesFile: "testdata/nodes_mixed.json",
		podsFile:  "testdata/pods_mixed.json",
	}

	r, err := Collect(runner, Options{RestartThreshold: 5})
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if len(r.Nodes) != 3 {
		t.Fatalf("want 3 nodes, got %d", len(r.Nodes))
	}
	if got := r.HealthyNodes(); got != 1 {
		t.Errorf("want 1 healthy node (node-a only), got %d", got)
	}

	byName := map[string]NodeHealth{}
	for _, n := range r.Nodes {
		byName[n.Name] = n
	}

	if !byName["node-a"].Healthy() {
		t.Errorf("node-a should be healthy, got %+v", byName["node-a"])
	}
	if byName["node-a"].PodCount != 2 {
		t.Errorf("node-a should have 2 pods scheduled, got %d", byName["node-a"].PodCount)
	}
	if byName["node-b"].Healthy() {
		t.Errorf("node-b should be unhealthy (MemoryPressure), got %+v", byName["node-b"])
	}
	if byName["node-c"].Healthy() {
		t.Errorf("node-c should be unhealthy (not Ready), got %+v", byName["node-c"])
	}

	// worker-7c88-fghij: 9 restarts >= threshold 5 -> flagged
	// migrator-abc12: Pending -> flagged
	// api and cache pods: healthy -> not flagged
	if len(r.ProblemPods) != 2 {
		t.Fatalf("want 2 problem pods, got %d: %+v", len(r.ProblemPods), r.ProblemPods)
	}

	problemNames := map[string]PodIssue{}
	for _, p := range r.ProblemPods {
		problemNames[p.Name] = p
	}

	worker, ok := problemNames["worker-7c88-fghij"]
	if !ok {
		t.Fatal("expected worker-7c88-fghij to be flagged")
	}
	if worker.Reason != "CrashLoopBackOff" {
		t.Errorf("want reason CrashLoopBackOff, got %q", worker.Reason)
	}
	if worker.Restarts != 9 {
		t.Errorf("want 9 restarts, got %d", worker.Restarts)
	}

	migrator, ok := problemNames["migrator-abc12"]
	if !ok {
		t.Fatal("expected migrator-abc12 to be flagged")
	}
	if migrator.Phase != "Pending" {
		t.Errorf("want phase Pending, got %q", migrator.Phase)
	}

	if r.TotalPods != 4 {
		t.Errorf("want 4 total pods, got %d", r.TotalPods)
	}
}

func TestCollect_RestartThresholdIsConfigurable(t *testing.T) {
	runner := fakeRunner{
		nodesFile: "testdata/nodes_mixed.json",
		podsFile:  "testdata/pods_mixed.json",
	}

	// Raise the bar above 9 restarts: the crash-looping pod should no
	// longer be flagged on restart count alone (it's still flagged
	// because it's not Ready+Running cleanly is NOT checked here — only
	// restart threshold and phase are, so with a high threshold and a
	// Running phase it should drop out).
	r, err := Collect(runner, Options{RestartThreshold: 50})
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	for _, p := range r.ProblemPods {
		if p.Name == "worker-7c88-fghij" {
			t.Errorf("worker-7c88-fghij should not be flagged once threshold > its restart count, got %+v", p)
		}
	}
}

func TestCollect_RunnerError(t *testing.T) {
	runner := fakeRunner{nodesFile: "testdata/does-not-exist.json", podsFile: "testdata/pods_mixed.json"}
	if _, err := Collect(runner, Options{}); err == nil {
		t.Fatal("expected an error when the nodes fixture can't be read")
	}
}
