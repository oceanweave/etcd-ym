# Build the manager binary
FROM golang:1.17 as builder

RUN apt-get -y update && apt-get -y install upx

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
#RUN export GOPROXY="https://goproxy.cn" && go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY cmd/ cmd/
COPY pkg/ pkg/

## Build 这个注释的方法 构建失败 无法运行
#RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on &&\
#    go build -a -o manager main.go &&\
#    go build -a -o backup cmd/backup/main.go &&\
#    upx manager backup
# Build
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GO111MODULE=on
ENV GOPROXY="https://goproxy.cn"

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download && \
    go build -a -o manager main.go && \
    go build -a -o backup cmd/backup/main.go && \
    upx manager backup

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot as manager
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]

# backup target
# 多阶段构建 https://yeasy.gitbook.io/docker_practice/image/multistage-builds
# docker build --target backup -t dfy/etcd-operator-backup:v0.0.1 -f Dockerfile .
FROM gcr.io/distroless/static:nonroot as backup
WORKDIR /
COPY --from=builder /workspace/backup .
USER nonroot:nonroot

ENTRYPOINT ["/backup"]