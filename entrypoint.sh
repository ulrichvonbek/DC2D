#!/bin/sh
set -e
PASSWORD="${VNC_PASSWORD:-dc2d}"
Xvfb :0 -screen 0 640x512x24 &
export DISPLAY=:0
sleep 1
x11vnc -storepasswd "$PASSWORD" /root/.vncpasswd >/dev/null 2>&1
x11vnc -rfbauth /root/.vncpasswd -forever -shared -display :0 &
sleep 1
exec /opt/dc2d