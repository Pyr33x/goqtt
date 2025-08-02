FROM golang:1.24.1-alpine AS builder

RUN apk update
RUN apk add git gcc musl-dev sqlite-dev

WORKDIR /app

COPY go.mod go.sum config.yml ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=1 go build -o /app/goqtt ./cmd/goqtt

FROM alpine

WORKDIR /

COPY --from=builder /app/goqtt .
COPY --from=builder /app/config.yml .

ENTRYPOINT [ "/goqtt" ]
