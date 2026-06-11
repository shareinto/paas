FROM golang:1.25 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/paas-agent ./cmd/paas-agent

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/paas-agent /paas-agent
USER nonroot:nonroot
ENTRYPOINT ["/paas-agent"]
