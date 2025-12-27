#!/usr/bin/env python3
import socket
sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
sock.bind(('0.0.0.0', 8889))
print("Listening on port 8889...")
while True:
    data, addr = sock.recvfrom(1024)
    print(f"Received: {data.decode()}")

