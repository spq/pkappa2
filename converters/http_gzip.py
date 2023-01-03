#!/usr/bin/env python3
from pkappa2lib import *
from http.server import BaseHTTPRequestHandler
from http.client import parse_headers
from io import BytesIO
from urllib3 import HTTPResponse


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


class HTTPGzipConverter(Pkappa2Converter):

    def handle_stream(self, stream: Stream) -> Result:
        result_data = []
        for chunk in stream.Chunks:
            if chunk.Direction == Direction.CLIENTTOSERVER:
                try:
                    request = HTTPRequest(chunk.Content)
                    if request.error_code:
                        raise Exception(
                            f"{request.error_code} {request.error_message}".
                            encode())
                    data = chunk.Content
                except Exception as ex:
                    data = f"Unable to parse HTTP request: {ex}".encode()
            else:
                try:
                    # https://stackoverflow.com/a/52418392
                    header, body = chunk.Content.split(b"\r\n\r\n", 1)
                    header_stream = BytesIO(header)
                    requestline = header_stream.readline().split(b' ')
                    status = int(requestline[1])
                    headers = parse_headers(header_stream)

                    body_stream = BytesIO(body)
                    response = HTTPResponse(
                        body=body_stream,
                        headers=headers,
                        status=status,
                    )
                    data = header + b"\r\n\r\n" + response.data
                except Exception as ex:
                    data = f"Unable to parse HTTP response: {ex}".encode()
            result_data.append(StreamChunk(chunk.Direction, data))
        return Result(result_data)


if __name__ == "__main__":
    HTTPGzipConverter().run()
