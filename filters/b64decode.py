#!/usr/bin/env python3
import sys, json, base64
while 1:
 stream = json.loads(sys.stdin.buffer.readline())
 while 1:
  line = sys.stdin.buffer.readline().strip()
  if not line:
   break
  chunk = json.loads(line)
  data = base64.b64decode(chunk["data"])
  try:
   data = base64.b64decode(data)
  except:
   data = f"Unable to decode: {data:r}".encode()
  json.dump({"data": base64.b64encode(data).decode(), "dir": chunk["dir"]}, sys.stdout)
  print("")
 print("")
 print("{}", flush=True)
