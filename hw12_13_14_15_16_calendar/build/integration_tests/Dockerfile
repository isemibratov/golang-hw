# syntax=docker/dockerfile:1

FROM golang:1.19-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go test \
    -tags=integration \
    -c \
    -o /out/integration-tests \
    ./integration_tests

FROM alpine:3.22

RUN addgroup -S calendar && adduser -S -G calendar calendar

COPY --from=build /out/integration-tests /usr/local/bin/integration-tests

USER calendar

ENTRYPOINT ["/usr/local/bin/integration-tests"]
CMD ["-test.v", "-test.count=1", "-test.timeout=45s"]
