#!/usr/bin/python3
import base64
import json
import sys

lines = []
while 1:
    line = sys.stdin.readline().strip()
    if line != "":
        lines.append(json.loads(line))
        continue
    print(json.dumps({
        "Direction": "client-to-server",
        "Content": base64.b64encode(json.dumps({
            "converter": sys.argv[0],
            "info": lines[0],
            "data": lines[1:]
        }).encode()).decode()
    }))
    print()
    print("{}", flush=True)
    lines = []
