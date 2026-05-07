FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /ruptura-operator .

# ubi-micro: minimal Red Hat UBI — required for Red Hat OperatorHub preflight certification.
FROM registry.access.redhat.com/ubi9/ubi-micro

ARG VERSION=0.6.9

LABEL name="ruptura-operator" \
      vendor="Ruptura" \
      version="${VERSION}" \
      release="1" \
      summary="Kubernetes operator for the Ruptura predictive workload health platform" \
      description="Ruptura Operator manages the full lifecycle of a Ruptura deployment — PVC, Deployment, Service, and Route on OpenShift."

COPY --from=builder /ruptura-operator /ruptura-operator

USER 65532:65532

ENTRYPOINT ["/ruptura-operator"]
