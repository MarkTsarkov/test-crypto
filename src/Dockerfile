FROM golang:alpine AS builder

WORKDIR /app

COPY src/go.mod .
RUN go mod download

COPY src .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

FROM alpine:3.7

RUN apk add --no-cache bash
COPY --from=builder /app/main /
EXPOSE 8080

CMD ["/main"]