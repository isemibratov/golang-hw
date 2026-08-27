# syntax=docker/dockerfile:1

FROM golang:1.19-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -o /out/calendar_storer \
    ./cmd/calendar_storer

FROM alpine:3.22

RUN addgroup -S calendar && adduser -S -G calendar calendar

COPY --from=build /out/calendar_storer /usr/local/bin/calendar_storer
COPY configs/storer.toml /etc/calendar/storer.toml

USER calendar

ENTRYPOINT ["/usr/local/bin/calendar_storer"]
CMD ["--config=/etc/calendar/storer.toml"]
