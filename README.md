# kube-node-doctor

A small, dependency-free CLI that gives you a one-screen health check of a
Kubernetes cluster: which nodes are `Ready` and pressure-free, and which
pods are crash-looping, stuck `Pending`, or restarting more than you'd
like.

```
$ kube-node-doctor
kube-node-doctor report — 2026-10-15 15:27:19 UTC

NODE    READY  PRESSURE        PODS
node-a  yes    -               2
node-b  yes    MemoryPressure  1
node-c  NO     -               0

2 pod(s) need attention:

NAMESPACE  POD                NODE    PHASE    RESTARTS  REASON
data       migrator-abc12             Pending  0         Pending
default    worker-7c88-fghij  node-b  Running  9         CrashLoopBackOff

Summary: 1/3 nodes healthy, 2/4 pods flagged.
```

## Why it exists

Most "is my cluster OK" checks start with `kubectl get nodes` and
`kubectl get pods --all-namespaces`, then a human eyeballs two wide tables
for `NotReady` and `CrashLoopBackOff`. This tool does that eyeballing for
you and gives it an exit code, so it can sit in a pre-deploy check, a
cron job, or a Slack bot just as easily as a terminal.

## Design choices

- **No `client-go`, on purpose.** `kube-node-doctor` shells out to
  `kubectl get ... -o json` and decodes only the fields it needs. That
  keeps the binary tiny, keeps it compatible with whatever API version
  the cluster's own `kubectl` understands, and — more importantly for
  this repo — means the whole thing builds and tests with **zero
  external dependencies** (`go.mod` has no `require` block at all).
- **The boundary is an interface.** All cluster access goes through a
  single `Runner` interface (`internal/k8s/runner.go`). Production code
  uses `KubectlRunner`, which really execs `kubectl`; tests use a fake
  that serves canned JSON fixtures. That's what makes `go test ./...`
  pass without a real cluster, a kubeconfig, or network access — see
  `internal/report/report_test.go`.
- **Health rules are explicit, not magic.** A node is unhealthy if its
  `Ready` condition isn't `True`, or if `MemoryPressure` / `DiskPressure`
  / `PIDPressure` / `NetworkUnavailable` is `True`. A pod is flagged if
  it's `Pending`/`Failed`/`Unknown`, or any container has restarted at
  least `--restart-threshold` times (default 5). No metrics-server
  dependency, no heuristics beyond what the API already tells you.

## Install / run

```bash
go build -o bin/kube-node-doctor ./cmd/kube-node-doctor
./bin/kube-node-doctor                        # table output, current kube context
./bin/kube-node-doctor --namespace prod        # scope to one namespace
./bin/kube-node-doctor --json                  # machine-readable output
./bin/kube-node-doctor --fail-on-issue         # exit 1 if anything's unhealthy — CI-friendly
```

Or via Docker (image bundles a pinned `kubectl`, runs as non-root on
`distroless`):

```bash
docker build -t kube-node-doctor .
docker run --rm -v ~/.kube:/home/nonroot/.kube:ro kube-node-doctor
```

## Development

```bash
make lint   # gofmt + go vet + go test -cover
make build  # ./bin/kube-node-doctor
```

CI (`.github/workflows/ci.yml`) runs `gofmt -l`, `go vet`, `go test
-race -cover`, a build, and a Docker build on every push and PR.

## Project layout

```
cmd/kube-node-doctor/   CLI entrypoint — flag parsing only
internal/k8s/           Runner interface + kubectl-exec implementation + API types
internal/report/        Health-report logic (pure, fully unit-tested) + table/JSON rendering
```

## Possible next steps

- `--selector` to scope pods by label
- a `watch` mode that polls on an interval and only prints on change
- optional Prometheus `/metrics` endpoint for the `--fail-on-issue`
  boolean, for scraping instead of scripting around exit codes
