###################
## Build go binary
FROM registry.access.redhat.com/ubi9/go-toolset:1.22.9-1736729788 AS build

ARG BUILD_ARGS

USER root

WORKDIR /build

COPY go.mod .
COPY go.sum .

RUN --mount=type=cache,mode=0755,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,mode=0755,target=/go/pkg/mod GOOS=linux make build.local BUILD_ARGS="${BUILD_ARGS}"


############
## Licenses
FROM registry.access.redhat.com/ubi9/go-toolset:1.22.9-1736729788 AS licenses

ADD . /app
WORKDIR /app

RUN go install github.com/google/go-licenses@v1.6.0
RUN ${HOME}/go/bin/go-licenses save --save_path /tmp/licenses ./...


########################
## Create runtime image
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.2

ARG release=main
ARG version=latest

LABEL com.redhat.component ccx-exporter
LABEL description "Pipeline processors to export assisted installer events to s3"
LABEL summary "Pipeline processors to export assisted installer events to s3"
LABEL io.k8s.description "Pipeline processors to export assisted installer events to s3"
LABEL distribution-scope public
LABEL name ccx-exporter
LABEL release ${release}
LABEL version ${version}
LABEL url https://github.com/openshift-assisted/ccx-exporter
LABEL vendor "Red Hat, Inc."
LABEL maintainer "Red Hat"

COPY --from=build /build/build/ccx-exporter /usr/bin/ccx-exporter
COPY --from=licenses /tmp/licenses /licenses

# Metrics port
EXPOSE 7777

USER 1001:1001

ENTRYPOINT ["/usr/bin/ccx-exporter"]
