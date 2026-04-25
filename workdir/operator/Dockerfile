FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /ohe-operator .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /ohe-operator /ohe-operator
ENTRYPOINT ["/ohe-operator"]
