# syntax=docker/dockerfile:1

FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    go mod download
COPY app/auth ./app/auth
COPY app/common ./app/common
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=flash-mall-go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags timetzdata -o /out/auth-api ./app/auth/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/auth-api /app/auth-api

EXPOSE 8890
ENTRYPOINT ["/app/auth-api"]
