FROM node:24-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /backend .

FROM alpine:3.22
WORKDIR /app
COPY --from=build /backend /app/backend
COPY --from=frontend /src/dist /static
ENV STATIC_DIR=/static
EXPOSE 8000
CMD ["/app/backend"]
