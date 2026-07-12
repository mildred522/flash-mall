# syntax=docker/dockerfile:1

FROM node:20-alpine AS frontend
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web/ ./web/
RUN cd web && npm run build

FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    go mod download
COPY app/common ./app/common
COPY app/entry ./app/entry
COPY app/product/rpc/product ./app/product/rpc/product
COPY app/product/rpc/productclient ./app/product/rpc/productclient
COPY app/order/rpc/order ./app/order/rpc/order
COPY app/order/rpc/orderclient ./app/order/rpc/orderclient
COPY app/inventory/kitex/kitex_gen ./app/inventory/kitex/kitex_gen
COPY --from=frontend /src/app/entry/api/internal/handler/web/ ./app/entry/api/internal/handler/web/
RUN --mount=type=cache,id=flash-mall-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=flash-mall-go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -tags timetzdata -o /out/entry-api ./app/entry/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/entry-api /app/entry-api

EXPOSE 8888 6060 9090
ENTRYPOINT ["/app/entry-api"]
