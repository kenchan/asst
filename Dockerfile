FROM golang:1.21 AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o asst

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/asst .

CMD ["./asst"]
