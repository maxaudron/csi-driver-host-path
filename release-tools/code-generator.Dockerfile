FROM golang:1.11.2

ENV GO111MODULE=off

RUN go get k8s.io/code-generator; exit 0
RUN go get k8s.io/apimachinery; exit 0
RUN go get github.com/spf13/pflag && \
    go get k8s.io/gengo/args && \
    go get k8s.io/gengo/examples/defaulter-gen/generators && \
    go get k8s.io/klog && \
    go get k8s.io/gengo/generator && \
    go get k8s.io/gengo/namer && \
    go get k8s.io/gengo/types && \
    go get k8s.io/gengo/examples/deepcopy-gen/generators

ARG repo="${GOPATH}/src/github.com/maxaudron/zfs-csi-driver"

RUN mkdir -p $repo

WORKDIR $GOPATH/src/k8s.io/code-generator

VOLUME $repo
