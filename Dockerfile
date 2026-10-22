# golang:1.27.3-trixie
FROM golang@sha256:fbb7ae40b62ff8afe31662df570d39b4f5052ac32b90dae8e60d7657a20a74f4 AS builder

ARG TARGETOS
ARG TARGETARCH

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

WORKDIR /go/src/github.com/kauche/cloud-run-service-router-xds

COPY . .

RUN CGO_ENABLED=0 go build \
        -a \
        -trimpath \
        -ldflags "-s -w -extldflags -static" \
        -o /usr/bin/cloud-run-service-router-xds \
        .

## Runtime

# gcr.io/distroless/base-debian13:latest
FROM gcr.io/distroless/base@sha256:44a80dc28b49f47396e72ab2dc644b3f849880335de09625762faed207bb891d
COPY --from=builder /usr/bin/cloud-run-service-router-xds /usr/bin/cloud-run-service-router-xds

CMD ["/usr/bin/cloud-run-service-router-xds"]
