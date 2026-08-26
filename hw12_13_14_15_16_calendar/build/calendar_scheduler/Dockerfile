# syntax=docker/dockerfile:1

FROM golang:1.19-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/calendar_scheduler \
    ./cmd/calendar_scheduler

FROM alpine:3.22

RUN addgroup -S calendar && adduser -S -G calendar calendar

COPY --from=build /out/calendar_scheduler /usr/local/bin/calendar_scheduler
COPY configs/scheduler.toml /etc/calendar/scheduler.toml

USER calendar

EXPOSE 8081

ENTRYPOINT ["/usr/local/bin/calendar_scheduler"]
CMD ["--config=/etc/calendar/scheduler.toml"]
