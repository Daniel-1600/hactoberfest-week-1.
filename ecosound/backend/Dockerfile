# CGO is required by go-sqlite3. Keep build/runtime distributions compatible.
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -buildvcs=false -trimpath -o /out/ecosound-server ./cmd/server

FROM debian:bookworm-slim
# Audio uploads are decoded locally before BirdNET is called on Modal.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/ecosound-server /app/ecosound-server
ENV BACKEND_HOST=0.0.0.0 BACKEND_PORT=10000
EXPOSE 10000
CMD ["/app/ecosound-server"]
