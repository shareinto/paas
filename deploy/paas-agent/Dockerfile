ARG GO_IMAGE=golang:1.25
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot

FROM ${GO_IMAGE} AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/paas-agent ./cmd/paas-agent

FROM ${RUNTIME_IMAGE}

COPY --from=builder /out/paas-agent /paas-agent
USER nonroot:nonroot
ENTRYPOINT ["/paas-agent"]
