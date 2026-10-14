# Timetrack — build multi-stage: frontend (Vite) → backend (Go, com o dist
# embarcado) → runtime mínimo. Um único container de app (D-04/D-10).

FROM node:22-alpine AS front
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS back
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# substitui o placeholder pelo dist real antes do embed
COPY --from=front /app/frontend/dist ./internal/api/dist
RUN CGO_ENABLED=0 go build -o /bin/timetrack ./cmd/server

FROM alpine:3.21
RUN adduser -D -u 10001 timetrack
USER timetrack
COPY --from=back /bin/timetrack /usr/local/bin/timetrack
EXPOSE 8080
ENTRYPOINT ["timetrack"]
