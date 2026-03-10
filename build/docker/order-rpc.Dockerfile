FROM golang:1.24 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/order-rpc ./app/order/rpc

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/order-rpc /app/order-rpc

EXPOSE 8090 6061 9091
ENTRYPOINT ["/app/order-rpc"]
