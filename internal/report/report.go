// Package report turns raw kubectl output into a cluster health report:
// which nodes are under pressure or not Ready, and which pods are crash
// looping, stuck pending, or restarting more than a given threshold.
package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/ashsharon/kube-node-doctor/internal/k8s"
)

// pressureConditions are the node conditions that indicate trouble when
// they report "True" (unlike Ready, where "True" is the healthy state).
var pressureConditions = map[string]bool{
	"MemoryPressure":     true,
	"DiskPressure":       true,
	"PIDPressure":        true,
	"NetworkUnavailable": true,
}

// NodeHealth summarizes one node.
type NodeHealth struct {
	Name      string   `json:"name"`
	Ready     bool     `json:"ready"`
	Pressures []string `json:"pressures,omitempty"`
	PodCount  int      `json:"podCount"`
}

func (n NodeHealth) Healthy() bool {
	return n.Ready && len(n.Pressures) == 0
}

// PodIssue describes one pod that needs a human's attention.
type PodIssue struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Node      string `json:"node"`
	Phase     string `json:"phase"`
	Reason    string `json:"reason"`
	Restarts  int    `json:"restarts"`
}

// Report is the full result of a Collect call.
type Report struct {
	GeneratedAt time.Time    `json:"generatedAt"`
	Nodes       []NodeHealth `json:"nodes"`
	ProblemPods []PodIssue   `json:"problemPods"`
	TotalPods   int          `json:"totalPods"`
}

// HealthyNodes returns how many nodes are fully healthy.
func (r Report) HealthyNodes() int {
	n := 0
	for _, node := range r.Nodes {
		if node.Healthy() {
			n++
		}
	}
	return n
}

// Options configures a Collect call.
type Options struct {
	// Namespace restricts the pod scan; empty means all namespaces.
	Namespace string
	// RestartThreshold flags a pod once any container has restarted at
	// least this many times.
	RestartThreshold int
}

// Collect fetches nodes and pods through runner and builds a Report.
func Collect(runner k8s.Runner, opts Options) (*Report, error) {
	nodes, err := fetchNodes(runner)
	if err != nil {
		return nil, fmt.Errorf("fetching nodes: %w", err)
	}
	pods, err := fetchPods(runner, opts.Namespace)
	if err != nil {
		return nil, fmt.Errorf("fetching pods: %w", err)
	}

	health := make(map[string]*NodeHealth, len(nodes.Items))
	for _, n := range nodes.Items {
		nh := &NodeHealth{Name: n.Metadata.Name, Ready: false}
		for _, c := range n.Status.Conditions {
			switch {
			case c.Type == "Ready":
				nh.Ready = c.Status == "True"
			case pressureConditions[c.Type] && c.Status == "True":
				nh.Pressures = append(nh.Pressures, c.Type)
			}
		}
		health[nh.Name] = nh
	}

	threshold := opts.RestartThreshold
	if threshold <= 0 {
		threshold = 5
	}

	var problems []PodIssue
	for _, p := range pods.Items {
		if nh, ok := health[p.Spec.NodeName]; ok {
			nh.PodCount++
		}

		maxRestarts := 0
		reason := ""
		for _, cs := range p.Status.ContainerStatuses {
			if cs.RestartCount > maxRestarts {
				maxRestarts = cs.RestartCount
			}
			if r := cs.WaitingReason(); r != "" && reason == "" {
				reason = r
			}
		}

		flagged := false
		switch p.Status.Phase {
		case "Failed", "Unknown":
			flagged = true
			if reason == "" {
				reason = p.Status.Phase
			}
		case "Pending":
			flagged = true
			if reason == "" {
				reason = "Pending"
			}
		}
		if maxRestarts >= threshold {
			flagged = true
			if reason == "" {
				reason = "high restart count"
			}
		}

		if flagged {
			problems = append(problems, PodIssue{
				Namespace: p.Metadata.Namespace,
				Name:      p.Metadata.Name,
				Node:      p.Spec.NodeName,
				Phase:     p.Status.Phase,
				Reason:    reason,
				Restarts:  maxRestarts,
			})
		}
	}

	sort.Slice(problems, func(i, j int) bool {
		if problems[i].Namespace != problems[j].Namespace {
			return problems[i].Namespace < problems[j].Namespace
		}
		return problems[i].Name < problems[j].Name
	})

	var nodeList []NodeHealth
	for _, nh := range health {
		nodeList = append(nodeList, *nh)
	}
	sort.Slice(nodeList, func(i, j int) bool { return nodeList[i].Name < nodeList[j].Name })

	return &Report{
		GeneratedAt: time.Now().UTC(),
		Nodes:       nodeList,
		ProblemPods: problems,
		TotalPods:   len(pods.Items),
	}, nil
}

func fetchNodes(runner k8s.Runner) (*k8s.NodeList, error) {
	out, err := runner.Run("get", "nodes", "-o", "json")
	if err != nil {
		return nil, err
	}
	var list k8s.NodeList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("decoding node list: %w", err)
	}
	return &list, nil
}

func fetchPods(runner k8s.Runner, namespace string) (*k8s.PodList, error) {
	args := []string{"get", "pods", "-o", "json"}
	if namespace == "" {
		args = append(args, "--all-namespaces")
	} else {
		args = append(args, "-n", namespace)
	}
	out, err := runner.Run(args...)
	if err != nil {
		return nil, err
	}
	var list k8s.PodList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("decoding pod list: %w", err)
	}
	return &list, nil
}
