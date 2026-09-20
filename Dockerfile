FROM golang:1.26.7 AS build
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential pkg-config libgl1-mesa-dev xorg-dev \
    xvfb libgl1-mesa-dri libglx-mesa0 xauth \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 xvfb-run -a go test ./... && CGO_ENABLED=1 go build -o /opt/dc2d .

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    xvfb x11vnc \
    libgl1 libgl1-mesa-dri libglx0 libegl1 \
    libx11-6 libxrandr2 libxcursor1 libxi6 libxinerama1 libxxf86vm1 \
    xauth \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /opt/dc2d /opt/dc2d
COPY entrypoint.sh /opt/entrypoint.sh
RUN chmod +x /opt/entrypoint.sh
EXPOSE 5900
ENTRYPOINT ["/opt/entrypoint.sh"]