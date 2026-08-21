# syntax=docker/dockerfile:1
# 多阶段构建：server(Go) + web(Vue) -> 精简运行时镜像

# ---- 构建 Go 服务端 ----
FROM golang:1.22-alpine AS server-build
WORKDIR /src
COPY server/go.mod ./go.mod
COPY server/ ./server/
RUN cd server && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ns-oauth2-server .

# ---- 构建 Vue 前端 ----
FROM node:20-alpine AS web-build
WORKDIR /web
COPY web/package*.json ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- 运行时 ----
FROM alpine:3.20
RUN adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=server-build /out/ns-oauth2-server /app/ns-oauth2-server
# 前端放默认静态目录 ../web/dist（相对 /app，即 /web/dist），与 NS_WEB_DIST 默认值一致
COPY --from=web-build /web/dist /web/dist
USER app
EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/app/ns-oauth2-server"]
