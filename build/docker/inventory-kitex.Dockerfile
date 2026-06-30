FROM golang:1.24 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/inventory-kitex ./app/inventory/kitex

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/inventory-kitex /app/inventory-kitex

EXPOSE 8093
ENTRYPOINT ["/app/inventory-kitex"]
