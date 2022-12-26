#!/usr/bin/env python3
import sys, json, base64
while 1:
    stream = json.loads(sys.stdin.buffer.readline())
    while 1:
        line = sys.stdin.buffer.readline().strip()
        if not line:
            break
        chunk = json.loads(line)
        data = base64.b64decode(chunk["Data"])
        # Filter payload code starts here.
        try:
            data = base64.b64decode(data)
        except Exception as ex:
            data = f"Unable to decode: {ex}".encode()
        # Filter payload code ends here.
        json.dump(
            {
                "Data": base64.b64encode(data).decode(),
                "Direction": chunk["Direction"]
            }, sys.stdout)
        print("")
    print("")
    print("{}", flush=True)
