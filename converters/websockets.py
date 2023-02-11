#!/usr/bin/env python3
from base64 import b64encode
from typing import Any, Dict, List, Union
from dataclasses import dataclass
from hashlib import sha1
from http.server import BaseHTTPRequestHandler
from http.client import parse_headers
from io import BytesIO
import zlib
import traceback
from urllib3 import HTTPResponse
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


@dataclass
class WebsocketFrame:
    Direction: Direction
    Header: bytearray
    Data: bytearray


class WebsocketConverter(Pkappa2Converter):

    websocket_key: Union[bytes, None]
    switched_protocols: bool
    websocket_deflate: bool
    websocket_deflate_decompressor: Dict[Direction, Any]
    websocket_message_fragmented_frames: List[WebsocketFrame]

    def unmask_websocket_frames(self, frame: WebsocketFrame) -> WebsocketFrame:
        # this frame is unmasked
        if frame.Header[1] & 0x80 == 0:
            return frame

        unmasked = [
            frame.Data[4 + i] ^ frame.Data[i % 4]
            for i in range(len(frame.Data) - 4)
        ]
        # remove mask bit
        frame.Header[1] = frame.Header[1] & 0x7F
        return WebsocketFrame(frame.Direction, frame.Header,
                              bytearray(unmasked))

    def handle_websocket_permessage_deflate(
            self, frame: WebsocketFrame) -> Union[WebsocketFrame, None]:
        opcode = frame.Header[0] & 0x0F
        # control frames are not compressed
        if opcode & 0x08 != 0:
            return frame

        # handle fragmented messages
        if frame.Header[0] & 0x80 == 0:  # FIN bit not set
            self.websocket_message_fragmented_frames.append(frame)
            if len(self.websocket_message_fragmented_frames
                   ) > 0 and opcode != 0:
                self.websocket_message_fragmented_frames = []
                raise Exception("Invalid fragmented message")
            if len(self.websocket_message_fragmented_frames
                   ) > 50:  # arbitrary limit
                self.websocket_message_fragmented_frames = []
                raise Exception("Fragmented message too long")
            return None

        if len(self.websocket_message_fragmented_frames) > 0:
            if opcode != 0:
                self.websocket_message_fragmented_frames = []
                raise Exception("Invalid fragmented message")
            # this is the last frame of a fragmented message
            self.websocket_message_fragmented_frames.append(frame)
            frame = WebsocketFrame(
                Direction=frame.Direction,
                Header=self.websocket_message_fragmented_frames[0].Header,
                Data=bytearray(b''.join([
                    f.Data for f in self.websocket_message_fragmented_frames
                ])))
            frame.Header[0] |= 0x80  # set FIN bit
            # datalength in the header is wrong now, but we don't care
            self.websocket_message_fragmented_frames = []

        # only the first frame of a fragmented message has the RSV1 bit set
        if frame.Header[
                0] & 0x40 == 0:  # RSV1 "Per-Message Compressed" bit not set
            return frame

        data = frame.Data + b'\x00\x00\xff\xff'
        data = self.websocket_deflate_decompressor[frame.Direction].decompress(
            data)
        frame.Header[0] = frame.Header[0] & 0xBF  # remove RSV1 bit
        return WebsocketFrame(frame.Direction, frame.Header, bytearray(data))

    def handle_websocket_frames(self, chunk: StreamChunk) -> bytes:
        try:
            frames = []
            frame = bytearray(chunk.Content)
            while len(frame) > 0:
                data_length = frame[1] & 0x7F
                mask_offset = 2
                if data_length == 126:
                    mask_offset = 4
                    data_length = int.from_bytes(frame[2:4], byteorder="big")
                elif data_length == 127:
                    mask_offset = 10
                    data_length = int.from_bytes(frame[2:10], byteorder="big")

                data_offset = mask_offset
                # frame masked?
                if frame[1] & 0x80 != 0:
                    data_offset += 4

                websocket_frame = WebsocketFrame(
                    Direction=chunk.Direction,
                    Header=frame[:mask_offset],
                    Data=frame[mask_offset:data_offset + data_length])
                websocket_frame = self.unmask_websocket_frames(websocket_frame)
                if self.websocket_deflate:
                    websocket_frame = self.handle_websocket_permessage_deflate(
                        websocket_frame)
                    if websocket_frame is None:
                        continue

                frames.append(websocket_frame.Header +
                              bytes(websocket_frame.Data))
                frame = frame[data_offset + data_length:]
            return b''.join(frames)
        except Exception as ex:
            self.log(f"Error while handling websocket frame: {ex}")
            self.log(traceback.format_exc())

            raise Exception(
                f"Error while handling websocket frame: {ex}") from ex

    def handle_permessage_deflate_extension(
            self, websocket_deflate_parameters: Dict[str, Union[bool,
                                                                str]]) -> None:
        self.websocket_deflate = True
        self.websocket_message_fragmented_frames = []
        window_bits = 15
        if 'server_max_window_bits' in websocket_deflate_parameters:
            window_bits = int(
                websocket_deflate_parameters['server_max_window_bits'])
        self.websocket_deflate_decompressor[
            Direction.SERVERTOCLIENT] = zlib.decompressobj(wbits=-window_bits)
        window_bits = 15
        if 'client_max_window_bits' in websocket_deflate_parameters:
            window_bits = int(
                websocket_deflate_parameters['client_max_window_bits'])
        self.websocket_deflate_decompressor[
            Direction.CLIENTTOSERVER] = zlib.decompressobj(wbits=-window_bits)

    def handle_protocol_switch(self, chunk: StreamChunk) -> bytes:
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
                    websocket_key = request.headers.get(
                        "Sec-WebSocket-Key", None)
                    if websocket_key is None:
                        raise Exception("No websocket key found")
                    self.websocket_key = websocket_key.encode()
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

                data: bytes = response.data
                if response.headers.get("Connection").lower(
                ) == "upgrade" and response.headers.get(
                        "Upgrade").lower() == "websocket":
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

                    # Decode extensions header
                    # Sec-WebSocket-Extensions: extension-name; param1=value1; param2="value2", extension-name2; param1, extension-name3, extension-name4; param1=value1
                    raw_extensions = response.headers.get(
                        "Sec-WebSocket-Extensions")
                    extensions: Dict[str, Dict[str, Union[str, bool]]] = {}
                    if raw_extensions:
                        raw_extensions = map(lambda s: s.strip().lower(),
                                             raw_extensions.split(","))
                        for extension in raw_extensions:
                            extension, raw_params = extension.split(
                                ";", 1) if ";" in extension else (extension,
                                                                  '')
                            params: Dict[str, Union[str, bool]] = {}
                            raw_params = filter(
                                lambda p: len(p) != 0,
                                map(lambda p: p.strip(),
                                    raw_params.split(";")))
                            for param in raw_params:
                                param = param.split("=", 1)
                                if len(param) == 1:
                                    params[param[0]] = True
                                else:
                                    if param[1].startswith(
                                            '"') and param[1].endswith('"'):
                                        param[1] = param[1][1:-1]
                                    params[param[0]] = param[1]
                            extensions[extension] = params
                    if extensions and "permessage-deflate" in extensions:
                        self.handle_permessage_deflate_extension(
                            extensions["permessage-deflate"])
                    elif len(extensions) > 0:
                        self.log(f"Unsupported extensions: {extensions}")

                    if len(data) > 0:
                        data = self.handle_websocket_frames(
                            StreamChunk(chunk.Direction, data))

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
        self.websocket_deflate = False
        self.websocket_deflate_decompressor = {}
        self.websocket_message_fragmented_frames = []
        for chunk in stream.Chunks:
            try:
                if not self.switched_protocols:
                    data = self.handle_protocol_switch(chunk)
                else:
                    data = self.handle_websocket_frames(chunk)
            except Exception as ex:
                data = str(ex).encode()
            result_data.append(StreamChunk(chunk.Direction, data))
        return Result(result_data)


if __name__ == "__main__":
    WebsocketConverter().run()
