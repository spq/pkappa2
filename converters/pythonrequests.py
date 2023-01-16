#!/usr/bin/env python3
from io import BytesIO
from http.server import BaseHTTPRequestHandler
from pkappa2lib import *


# https://stackoverflow.com/questions/4685217/parse-raw-http-headers
class HTTPRequest(BaseHTTPRequestHandler):

    def __init__(self, request_text: str):
        self.rfile = BytesIO(request_text)
        self.raw_requestline = self.rfile.readline()
        self.error_code = self.error_message = None
        self.parse_request()

    def send_error(self, code, message):
        self.error_code = code
        self.error_message = message


class PythonRequestsConverter(Pkappa2Converter):

    def handle_stream(self, stream: Stream) -> Result:
        typ = ''
        if stream.Metadata.Protocol == Protocol.UDP:
            typ = ', typ = "udp"'
        output = f'''#!/usr/bin/env python3
import requests

# Generated from stream {stream.Metadata.StreamID}
s = requests.Session()

'''
        for i, chunk in enumerate(stream.Chunks):
            if chunk.Direction == Direction.CLIENTTOSERVER:
                try:
                    request = HTTPRequest(chunk.Content)
                    if request.error_code:
                        raise Exception(
                            f"{request.error_code} {request.error_message}".
                            encode())

                    port = ''
                    if stream.Metadata.ServerPort != 80:
                        port = f':{stream.Metadata.ServerPort}'
                    data = request.rfile.read()
                    headers = {}
                    for k, v in request.headers.items():
                        headers[k] = v
                    output += f'# Chunk {i}\n'
                    output += f'r = s.request({request.command!r}, "http://{stream.Metadata.ServerHost}{port}{request.path}"'
                    if len(headers) > 0:
                        output += f', headers={headers}'
                    if len(data) > 0:
                        output += f', data={data}'
                    output += ')\n'
                except Exception as ex:
                    output += f"# Unable to parse HTTP request: {ex}\n"

        return Result([StreamChunk(Direction.CLIENTTOSERVER, output.encode())])


if __name__ == "__main__":
    PythonRequestsConverter().run()
