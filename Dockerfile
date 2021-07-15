FROM golang:1.15-alpine as build

ENV GOARCH=amd64

WORKDIR /src

COPY go.sum .
COPY go.mod .

RUN export GOPROXY=https://goproxy.cn,direct && go mod download

COPY . .

RUN go build -o app main.go

FROM alpine:3

RUN apk update && apk update ca-certificates

WORKDIR /go/bin

COPY --chown=0:0 --from=build /src/app /go/bin/app

ENTRYPOINT ["./app"]
