FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22
RUN adduser -D -u 10001 app && mkdir -p /data && chown app:app /data
USER app
COPY --from=build /out/server /app/server
ENV DB_PATH=/data/subscription-hub.db
EXPOSE 8080
ENTRYPOINT ["/app/server"]
