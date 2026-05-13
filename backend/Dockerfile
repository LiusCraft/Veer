FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY --from=frontend-builder /app/frontend/dist ./dist/
COPY . ./
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o veer .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates sqlite-libs
WORKDIR /app
COPY --from=builder /app/veer .
COPY config-manager.yaml ./config-manager.yaml
EXPOSE 8080
CMD ["./veer"]
