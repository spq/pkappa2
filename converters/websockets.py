#!/usr/bin/env python3
from pkappa2lib import *
from http.server import BaseHTTPRequestHandler
from http.client import parse_headers
from io import BytesIO
from urllib3 import HTTPResponse
from base64 import b64encode
from hashlib import sha1


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


class WebsocketConverter(Pkappa2Converter):

    def unmask_websocket_frames(self, frame: bytes):
        try:
            frames = []
            frame = bytearray(frame)
            while len(frame) > 0:
                data_length = frame[1] & 0x7F
                mask_offset = 2
                if data_length == 126:
                    mask_offset = 4
                    data_length = int.from_bytes(frame[2:4], byteorder="big")
                elif data_length == 127:
                    mask_offset = 10
                    data_length = int.from_bytes(frame[2:10], byteorder="big")
                if frame[1] & 0x80 == 0:
                    frames.append(frame[:mask_offset + data_length])
                    frame = frame[mask_offset + data_length:]
                    continue
                mask = frame[mask_offset:mask_offset + 4]
                unmasked = [
                    frame[mask_offset + 4 + i] ^ mask[i % 4]
                    for i in range(data_length)
                ]
                frame[1] = frame[1] & 0x7F
                frames.append(frame[:mask_offset] + bytes(unmasked))
                frame = frame[mask_offset + 4 + data_length:]
            return b''.join(frames)
        except Exception as ex:
            raise Exception(
                f"Unable to unmask websocket frame: {ex}".encode()) from ex

    def handle_protocol_switch(self, chunk: StreamChunk):
        if chunk.Direction == Direction.CLIENTTOSERVER:
            try:
                request = HTTPRequest(chunk.Content)
                if request.error_code:
                    raise Exception(
                        f"{request.error_code} {request.error_message}".encode(
                        ))
                if request.headers.get(
                        "Connection") == "Upgrade" and request.headers.get(
                            "Upgrade") == "websocket":
                    self.websocket_key = request.headers.get(
                        "Sec-WebSocket-Key", None)
                    if self.websocket_key is None:
                        raise Exception("No websocket key found")
                    self.websocket_key = self.websocket_key.encode()
                return chunk.Content
            except Exception as ex:
                raise Exception(
                    f"Unable to parse HTTP request: {ex}".encode()) from ex
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

                data = response.data
                if response.headers.get(
                        "Connection") == "Upgrade" and response.headers.get(
                            "Upgrade") == "websocket":
                    if not self.websocket_key:
                        raise Exception("No websocket key found")
                    expected_accept = b64encode(
                        sha1(self.websocket_key +
                             b"258EAFA5-E914-47DA-95CA-C5AB0DC85B11").digest()
                    ).decode()
                    if response.headers.get(
                            "Sec-WebSocket-Accept") != expected_accept:
                        raise Exception(
                            f"Invalid websocket key: {response.headers.get('Sec-WebSocket-Accept')} != {expected_accept}"
                        )
                    self.switched_protocols = True

                    if len(data) > 0:
                        data = self.unmask_websocket_frames(data)

                if len(data) > 0:
                    return header + b"\r\n\r\n" + data
                return header + b"\r\n\r\n"
            except Exception as ex:
                raise Exception(
                    f"Unable to parse HTTP response: {ex}".encode()) from ex

    def handle_stream(self, stream: Stream) -> Result:
        result_data = []
        self.websocket_key = None
        self.switched_protocols = False
        for chunk in stream.Chunks:
            try:
                if not self.switched_protocols:
                    data = self.handle_protocol_switch(chunk)
                else:
                    data = self.unmask_websocket_frames(chunk.Content)
            except Exception as ex:
                data = str(ex).encode()
            result_data.append(StreamChunk(chunk.Direction, data))
        return Result(result_data)


if __name__ == "__main__":
    WebsocketConverter().run()
