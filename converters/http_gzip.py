#!/usr/bin/env python3
from base64 import urlsafe_b64decode
from http.server import BaseHTTPRequestHandler
from http.client import parse_headers
from io import BytesIO
from urllib3 import HTTPResponse
import h2.frame_buffer
import hyperframe.frame
from hpack import Decoder
from pkappa2lib import *

# pip install h2


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


class HTTPConverter(Pkappa2Converter):

    SETTINGS_NAMES = {
        1: "HEADER_TABLE_SIZE",
        2: "ENABLE_PUSH",
        3: "MAX_CONCURRENT_STREAMS",
        4: "INITIAL_WINDOW_SIZE",
        5: "MAX_FRAME_SIZE",
        6: "MAX_HEADER_LIST_SIZE",
        8: "ENABLE_CONNECT_PROTOCOL",
    }

    def __init__(self):
        super().__init__()
        self.h2_client_buffer = None
        self.h2_server_buffer = None
        self.hpack_decoder = None
        self.h2_active = False

    def format_http2_frame(self, frame: hyperframe.frame.Frame) -> str:
        if isinstance(frame, hyperframe.frame.HeadersFrame):
            headers = self.hpack_decoder.decode(frame.data)
            return f"{frame} {headers}"
        elif isinstance(frame, hyperframe.frame.DataFrame):
            return f"{frame} {frame.data}"
        elif isinstance(frame, hyperframe.frame.SettingsFrame):
            settings = {
                self.SETTINGS_NAMES.get(k, k): v
                for k, v in frame.settings.items()
            }
            return f"{type(frame).__name__}(stream_id={frame.stream_id}, flags={frame.flags!r}): {settings}"
        return str(frame)

    def setup_http2_buffers(self):
        self.h2_server_buffer = h2.frame_buffer.FrameBuffer(server=True)
        self.h2_server_buffer.max_frame_size = 16384
        self.h2_client_buffer = h2.frame_buffer.FrameBuffer(server=False)
        self.h2_client_buffer.max_frame_size = 16384
        self.hpack_decoder = Decoder()

    def handle_http2_upgrade(self, request: HTTPRequest) -> List[StreamChunk]:
        self.setup_http2_buffers()
        settings = request.headers.get("HTTP2-Settings")
        if settings:
            f = hyperframe.frame.SettingsFrame(0)
            f.parse_body(urlsafe_b64decode(settings))
            return [
                StreamChunk(Direction.CLIENTTOSERVER,
                            self.format_http2_frame(f).encode() + b"\n")
            ]
        return []

    def handle_http2_init(self, chunk: bytes) -> List[StreamChunk]:
        self.setup_http2_buffers()
        self.h2_active = True
        return self.handle_http2_request(chunk)

    def handle_http2_request(self, chunk: bytes) -> List[StreamChunk]:
        if not self.h2_server_buffer:
            return [StreamChunk(Direction.CLIENTTOSERVER, chunk)]
        self.h2_server_buffer.add_data(chunk)
        # TODO: Update max_frame_size when observing SETTINGS frames updating it
        events = []
        for event in self.h2_server_buffer:
            events.append(
                StreamChunk(Direction.CLIENTTOSERVER,
                            self.format_http2_frame(event).encode() + b"\n"))
        return events

    def handle_http2_response(self, chunk: bytes) -> List[StreamChunk]:
        if not self.h2_client_buffer:
            return [StreamChunk(Direction.SERVERTOCLIENT, chunk)]
        self.h2_active = True
        self.h2_client_buffer.add_data(chunk)
        events = []
        for event in self.h2_client_buffer:
            events.append(
                StreamChunk(Direction.SERVERTOCLIENT,
                            self.format_http2_frame(event).encode() + b"\n"))
        return events

    def handle_stream(self, stream: Stream) -> Result:
        self.h2_active = False
        self.h2_client_buffer = None
        self.hpack_decoder = None
        self.h2_server_buffer = None

        result_data = []
        for chunk in stream.Chunks:
            if chunk.Direction == Direction.CLIENTTOSERVER:
                if self.h2_active:
                    result_data.extend(self.handle_http2_request(
                        chunk.Content))
                    continue
                try:
                    if chunk.Content.startswith(
                            b"PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"):
                        # HTTP/2
                        result_data.extend(
                            self.handle_http2_init(chunk.Content))
                        continue

                    request = HTTPRequest(chunk.Content)
                    if request.error_code:
                        raise Exception(
                            f"{request.error_code} {request.error_message}".
                            encode())

                    # https://httpwg.org/specs/rfc7540.html#discover-http
                    connection = request.headers.get("Connection")
                    if connection:
                        connection_headers = list(
                            map(lambda h: h.strip(), connection.split(",")))
                        if "Upgrade" in connection_headers and request.headers.get(
                                "Upgrade") == "h2c":
                            # HTTP/2
                            result_data.extend(
                                self.handle_http2_upgrade(request))
                            continue

                    # Just pass HTTP1 requests through untouched
                    data = chunk.Content
                except Exception as ex:
                    data = f"Unable to parse HTTP request: {ex}".encode()
            else:
                try:
                    if self.h2_active:
                        # HTTP/2
                        result_data.extend(
                            self.handle_http2_response(chunk.Content))
                        continue

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

                    if response.headers.get(
                            "Connection"
                    ) == "Upgrade" and response.headers.get(
                            "Upgrade") == "h2c":
                        # HTTP/2
                        if self.h2_server_buffer is None:
                            raise Exception("HTTP/2 upgrade request not found")

                        result_data.extend(
                            self.handle_http2_response(response.data))
                        continue

                    data = header + b"\r\n\r\n" + response.data
                except Exception as ex:
                    data = f"Unable to parse HTTP response: {ex}".encode()
            result_data.append(StreamChunk(chunk.Direction, data))
        return Result(result_data)


if __name__ == "__main__":
    HTTPConverter().run()
