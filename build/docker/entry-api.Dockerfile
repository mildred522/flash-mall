FROM node:20-alpine AS frontend
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web/ ./web/
RUN cd web && npm run build

FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/app/entry/api/internal/handler/web/ ./app/entry/api/internal/handler/web/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/entry-api ./app/entry/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/entry-api /app/entry-api

EXPOSE 8888 6060 9090
ENTRYPOINT ["/app/entry-api"]
