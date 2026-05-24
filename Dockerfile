FROM golang:1.21-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o veer ./backend/cmd/manager

FROM alpine:3.19
RUN apk --no-cache add ca-certificates sqlite-libs
WORKDIR /app
COPY --from=builder /app/veer .
COPY backend/config-manager.yaml ./config-manager.yaml
EXPOSE 8080
CMD ["./veer"]