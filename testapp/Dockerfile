FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o testapp .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/testapp /testapp
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/testapp"]
