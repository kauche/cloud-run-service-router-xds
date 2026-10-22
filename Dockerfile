FROM golang:1.27.3-trixie AS builder

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

FROM gcr.io/distroless/base@sha256:44a80dc28b49f47396e72ab2dc644b3f849880335de09625762faed207bb891d
COPY --from=builder /usr/bin/cloud-run-service-router-xds /usr/bin/cloud-run-service-router-xds

CMD ["/usr/bin/cloud-run-service-router-xds"]
