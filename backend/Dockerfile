# 多阶段构建: 前端 → 后端 → 运行

# 阶段1: 前端构建
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# 阶段2: 后端构建
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY --from=frontend-builder /app/frontend/dist ./dist/
COPY backend/ ./
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o veer .

# 阶段3: 运行镜像
FROM alpine:3.19
RUN apk --no-cache add ca-certificates sqlite-libs
WORKDIR /app
COPY --from=backend-builder /app/backend/veer .
COPY backend/config.yaml ./config.yaml
EXPOSE 8080
CMD ["./veer"]
