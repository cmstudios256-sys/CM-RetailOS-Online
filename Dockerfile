FROM node:20 AS web-builder
WORKDIR /app
COPY web/package*.json ./web/
RUN cd web && npm ci
COPY web ./web
RUN cd web && npm run build

FROM golang:1.25 AS api-builder
WORKDIR /src
COPY api/go.mod api/go.sum ./api/
WORKDIR /src/api
RUN go mod download
COPY api .
RUN CGO_ENABLED=0 go build -o /out/cm-retailos ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=api-builder /out/cm-retailos /app/cm-retailos
COPY --from=web-builder /app/web/dist /app/web/dist
EXPOSE 8080
ENV FRONTEND_DIR=/app/web/dist
USER nonroot:nonroot
ENTRYPOINT ["/app/cm-retailos"]
