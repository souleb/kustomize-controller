ARG GO_VERSION=1.20
ARG XX_VERSION=1.2.1

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine as builder

# Copy the build utilities.
COPY --from=xx / /

ARG TARGETPLATFORM

WORKDIR /workspace

# copy api submodule
COPY api/ api/

# copy modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache modules
RUN go mod download

# copy source code
COPY main.go main.go
COPY internal/ internal/

# build
ENV CGO_ENABLED=0
RUN xx-go build -trimpath -a -o kustomize-controller main.go

FROM registry.access.redhat.com/ubi9/ubi

ARG TARGETPLATFORM

RUN yum install -y ca-certificates

# Add Tini
ENV TINI_VERSION=v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /sbin/tini
RUN chmod +x /sbin/tini

COPY --from=builder /workspace/kustomize-controller /usr/local/bin/
COPY LICENSE /licenses/LICENSE

USER 65534:65534

ENV GNUPGHOME=/tmp

ENTRYPOINT [ "/sbin/tini", "--", "kustomize-controller" ]
