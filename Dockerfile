FROM golang:1.16-alpine as builder
ENV CGO_ENABLED=0
COPY . /code
RUN apk add make
RUN cd /code && make build

FROM alpine
COPY --from=builder /code/bot /bot
# config
VOLUME /configs
WORKDIR /
ENTRYPOINT ["/bot"]
