package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// RenderJSON writes the report as indented JSON.
func RenderJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// RenderTable writes a human-readable summary: a node table, a problem-pod
// table (if any), and a one-line verdict.
func RenderTable(w io.Writer, r *Report) {
	fmt.Fprintf(w, "kube-node-doctor report — %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 MST"))

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NODE\tREADY\tPRESSURE\tPODS")
	for _, n := range r.Nodes {
		pressure := "-"
		if len(n.Pressures) > 0 {
			pressure = strings.Join(n.Pressures, ",")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", n.Name, yesNo(n.Ready), pressure, n.PodCount)
	}
	tw.Flush()

	if len(r.ProblemPods) > 0 {
		fmt.Fprintf(w, "\n%d pod(s) need attention:\n\n", len(r.ProblemPods))
		tw = tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "NAMESPACE\tPOD\tNODE\tPHASE\tRESTARTS\tREASON")
		for _, p := range r.ProblemPods {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n", p.Namespace, p.Name, p.Node, p.Phase, p.Restarts, p.Reason)
		}
		tw.Flush()
	} else {
		fmt.Fprintln(w, "\nNo problem pods found.")
	}

	fmt.Fprintf(w, "\nSummary: %d/%d nodes healthy, %d/%d pods flagged.\n",
		r.HealthyNodes(), len(r.Nodes), len(r.ProblemPods), r.TotalPods)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "NO"
}
