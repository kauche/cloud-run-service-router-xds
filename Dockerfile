FROM golang:1.19.4-bullseye AS builder

ENV GO111MODULE=on
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR /go/src/github.com/kauche/cloud-run-service-router-xds

COPY . .

RUN CGO_ENABLED=0 go build \
        -a \
        -trimpath \
        -ldflags "-s -w -extldflags -static" \
        -mod=vendor \
        -o /usr/bin/app \
        .

## Runtime

FROM gcr.io/distroless/base:3c213222937de49881c57c476e64138a7809dc54
COPY --from=builder /usr/bin/app /usr/bin/app

CMD ["/usr/bin/app"]
