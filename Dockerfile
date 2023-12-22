FROM golang:1.21-alpine as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN go build cmd/nats-load-traffic/main.go

FROM alpine
RUN adduser -S -D -H -h /app appuser
USER appuser
COPY --from=builder /build/main /app/
WORKDIR /app

ENTRYPOINT ["./main"]
CMD ["--help"]
