package k8s

import "encoding/json"

// The structs below intentionally mirror only the fields of the Kubernetes
// core/v1 API that the health report needs. Decoding a narrow subset keeps
// the tool resilient to the extra fields different cluster/kubectl versions
// attach to these objects.

// NodeList is the shape of `kubectl get nodes -o json`.
type NodeList struct {
	Items []Node `json:"items"`
}

type Node struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Conditions  []NodeCondition   `json:"conditions"`
		Allocatable map[string]string `json:"allocatable"`
		NodeInfo    struct {
			KubeletVersion string `json:"kubeletVersion"`
			OSImage        string `json:"osImage"`
		} `json:"nodeInfo"`
	} `json:"status"`
}

type NodeCondition struct {
	Type   string `json:"type"`   // Ready, MemoryPressure, DiskPressure, PIDPressure, NetworkUnavailable
	Status string `json:"status"` // "True" | "False" | "Unknown"
}

// PodList is the shape of `kubectl get pods -o json` (optionally with
// --all-namespaces).
type PodList struct {
	Items []Pod `json:"items"`
}

type Pod struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		NodeName string `json:"nodeName"`
	} `json:"spec"`
	Status struct {
		Phase             string            `json:"phase"`
		ContainerStatuses []ContainerStatus `json:"containerStatuses"`
	} `json:"status"`
}

type ContainerStatus struct {
	Name         string                     `json:"name"`
	Ready        bool                       `json:"ready"`
	RestartCount int                        `json:"restartCount"`
	State        map[string]json.RawMessage `json:"state"` // key is "waiting" | "running" | "terminated"
}

// WaitingReason returns the reason field if the container is currently
// waiting (e.g. CrashLoopBackOff, ImagePullBackOff), or "" otherwise.
func (c ContainerStatus) WaitingReason() string {
	raw, ok := c.State["waiting"]
	if !ok {
		return ""
	}
	var w struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return ""
	}
	return w.Reason
}
