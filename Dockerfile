FROM golang:1.15 as build

ENV GOARCH=amd64 \
    CGO_ENABLED=0

WORKDIR /go/src

COPY go.sum .
COPY go.mod .

RUN export GOPROXY=https://goproxy.cn,direct && go mod download

COPY . .

RUN go build -o app main.go

FROM alpine:3

RUN set -ex && \
    apk --no-cache --update add \
        ca-certificates \
        wget \
        tzdata

WORKDIR /go/bin

COPY --chown=0:0 --from=build /go/src/app /go/bin/app

ENTRYPOINT ["./app"]
