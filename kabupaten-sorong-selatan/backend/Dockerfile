FROM golang:1.23-alpine AS build

WORKDIR /src
COPY backend/go.mod backend/go.sum* ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /agenda-monitor .

FROM alpine:3.20

WORKDIR /app
RUN adduser -D -g "" appuser && mkdir -p /app/uploads
COPY --from=build /agenda-monitor /app/agenda-monitor
COPY frontend /frontend
COPY database /database
RUN chown -R appuser:appuser /app/uploads
USER appuser

ENV PORT=8080
ENV FRONTEND_DIR=/frontend
ENV SCHEMA_FILE=/database/schema.sql
ENV UPLOAD_DIR=/app/uploads

EXPOSE 8080
CMD ["/app/agenda-monitor"]
