FROM golang:alpine as builder

RUN apk update && apk add git

COPY . kite-server
WORKDIR kite-server
#get dependancies
RUN go get -d -v

#build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/kite-server

FROM alpine:latest
LABEL maintainer="Claude Debieux <claude@get-code.ch>"

#EXPOSE 9090

RUN apk add --no-cache --update bash iputils tzdata

RUN cp /usr/share/zoneinfo/Europe/Zurich /etc/localtime
RUN echo "Europe/Zurich" > /etc/timezone

WORKDIR /kite-server
COPY --from=builder /go/bin/kite-server /kite-server/kite-server
ENTRYPOINT ["/kite-server/kite-server"]