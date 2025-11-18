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

# Install tools
RUN mkdir -p /tmp/rclone && \
    cd /tmp/rclone && \
    curl https://downloads.rclone.org/v1.71.2/rclone-v1.71.2-linux-amd64.zip --output rclone.zip && \
    unzip rclone.zip


############
## Licenses
FROM registry.access.redhat.com/ubi9/go-toolset:1.22.9-1736729788 AS licenses

ADD . /app
WORKDIR /app

RUN go install github.com/google/go-licenses@v1.6.0
RUN ${HOME}/go/bin/go-licenses save --save_path /tmp/licenses ./...


########################
## Create runtime image
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.5-1736404155

ARG release=main
ARG version=latest

LABEL com.redhat.component=ccx-exporter \
    description="Pipeline processors to export assisted installer events to s3" \
    summary="Pipeline processors to export assisted installer events to s3" \
    io.k8s.description="Pipeline processors to export assisted installer events to s3" \
    distribution-scope=public \
    name=ccx-exporter \
    release=${release} \
    version=${version} \
    url=https://github.com/openshift-assisted/ccx-exporter \
    vendor="Red Hat, Inc." \
    maintainer="Red Hat"

COPY --from=build /build/build/ccx-exporter /usr/bin/ccx-exporter

COPY --from=licenses /tmp/licenses /licenses

COPY --from=build /tmp/rclone/rclone-v1.71.2-linux-amd64/rclone /usr/bin/rclone
COPY scripts/ /opt/ccx-exporter/bin/
RUN chmod +x /opt/ccx-exporter/bin/sync.sh

# Metrics port
EXPOSE 7777

USER 1001:1001

ENTRYPOINT ["/usr/bin/ccx-exporter"]
