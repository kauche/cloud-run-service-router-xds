FROM golang:1.20.4-bullseye AS builder

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

FROM gcr.io/distroless/base@sha256:df13a91fd415eb192a75e2ef7eacf3bb5877bb05ce93064b91b83feef5431f37
COPY --from=builder /usr/bin/cloud-run-service-router-xds /usr/bin/cloud-run-service-router-xds

CMD ["/usr/bin/cloud-run-service-router-xds"]
