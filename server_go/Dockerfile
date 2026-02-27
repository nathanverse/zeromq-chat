FROM golang:1.21-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chat-ws .

FROM alpine:3.20

RUN addgroup -S app && adduser -S -G app app \
    && apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=build /out/chat-ws /app/chat-ws

USER app
EXPOSE 8080

CMD ["/app/chat-ws"]
