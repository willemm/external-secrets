FROM golang:1.17 as builder
ARG GOPROXY=https://proxy.golang.org

RUN curl http://pr-art.europe.stater.corp/artifactory/auto-local/certs/pr-root.cer | sed -e "s/\r//g" > /usr/local/share/ca-certificates/pr-root.crt \
 && update-ca-certificates

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download -x

# Copy the go source
COPY main.go main.go
COPY e2e/ e2e/
COPY apis/ apis/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o external-secrets main.go

FROM distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/external-secrets /bin/external-secrets

# Run as UID for nobody
USER 65532:65532

ENTRYPOINT ["/bin/external-secrets"]
