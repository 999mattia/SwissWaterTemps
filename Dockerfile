# syntax=docker/dockerfile:1

FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend
WORKDIR /src/web
COPY web/go.mod web/go.sum ./
RUN go mod download
COPY web/ ./
COPY --from=frontend /src/web/internal/ui/dist ./internal/ui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/swisswatertemps .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /out/swisswatertemps /swisswatertemps
ENV PORT=3000
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD ["/swisswatertemps", "-healthcheck"]
ENTRYPOINT ["/swisswatertemps"]
