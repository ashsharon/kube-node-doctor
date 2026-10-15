# --- build stage -----------------------------------------------------------
FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/kube-node-doctor ./cmd/kube-node-doctor

# --- kubectl stage -----------------------------------------------------------
# kube-node-doctor talks to the Kubernetes API through kubectl, so kubectl is
# installed here too, pulled straight from the official Kubernetes release
# server and pinned to a specific version (not a third-party image whose
# tags can be renamed or removed out from under this build).
FROM alpine:3.20 AS kubectl
ARG KUBECTL_VERSION=v1.30.5
RUN apk add --no-cache curl \
    && curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && chmod +x /usr/local/bin/kubectl

# --- runtime stage -----------------------------------------------------------
# distroless static: no shell, no package manager, just the binary + certs.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=kubectl /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=build /out/kube-node-doctor /usr/local/bin/kube-node-doctor

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/kube-node-doctor"]
