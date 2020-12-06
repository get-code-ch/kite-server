FROM golang:alpine as builder

RUN apk update && apk add git sed

ENV server 0.0.0.0
ENV port 4080

COPY . kite-server
COPY ./config/template.json kite-server/config/template.json
COPY ./config/telegram_fake.json kite-server/config/telegram_fake.json

WORKDIR kite-server

RUN sed -ie "s/##SERVER##/$server/g" ./config/template.json
RUN sed -ie "s/##PORT##/$port/g" ./config/template.json

RUN sed -i '/^replace/d' go.mod

RUN cp -v ./config/template.json /go/default.json
RUN cp -v ./config/telegram_fake.json /go/telegram_fake.json

#get dependancies
#you can also use dep
RUN go get -d -v
#build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/kite-server

FROM alpine:latest
LABEL maintainer="Claude Debieux <claude@get-code.ch>"

ENV port 4080

EXPOSE 4080

RUN apk add --no-cache --update bash iputils tzdata

RUN cp /usr/share/zoneinfo/Europe/Zurich /etc/localtime
RUN echo "Europe/Zurich" > /etc/timezone

WORKDIR /kite-server
COPY --from=builder /go/bin/kite-server /kite-server/kite-server
RUN mkdir -p /kite-server/config
COPY --from=builder /go/default.json /kite-server/config/default.json
COPY --from=builder /go/telegram_fake.json /kite-server/config/telegram_fake.json


#ADD style /connectedhost/style

ENTRYPOINT ["/kite-server/kite-server"]