# --- build stage -----------------------------------------------------------
FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/kube-node-doctor ./cmd/kube-node-doctor

# --- runtime stage -----------------------------------------------------------
# distroless static: no shell, no package manager, just the binary + certs.
# kube-node-doctor talks to the Kubernetes API through kubectl, so kubectl
# is installed here too, pinned to a specific stable release.
FROM bitnami/kubectl:1.30 AS kubectl

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/kubectl
COPY --from=build /out/kube-node-doctor /usr/local/bin/kube-node-doctor

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/kube-node-doctor"]
