FROM golang:1.20-alpine3.18 AS builder

ADD go.mod go.sum /redis-endpoint-operator/
WORKDIR /redis-endpoint-operator
RUN go mod download
ADD . /redis-endpoint-operator/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-extldflags=-static" -o redis-endpoint-operator .  && \
    ls -lah redis-endpoint-operator

FROM scratch
COPY --from=builder /redis-endpoint-operator/redis-endpoint-operator /redis-endpoint-operator

ENTRYPOINT ["/redis-endpoint-operator"]
CMD ["-master", "mymaster", "-service", "redis-master"]
