#!/usr/bin/env python3
from base64 import b64encode
from typing import Any, Dict, List, Optional, Union
from dataclasses import dataclass
from hashlib import sha1
import zlib
import traceback
from http_gzip import HTTPConverter, HTTPRequest, HTTPResponse
from pkappa2lib import StreamChunk, Direction, Result, Stream


@dataclass
class WebsocketFrame:
    Direction: Direction
    Header: bytearray
    Data: bytearray


class WebsocketConverter(HTTPConverter):

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

    def handle_websocket_frame(self, chunk: StreamChunk,
                               frame: WebsocketFrame) -> WebsocketFrame:
        """
        Handle a websocket frame and possibly return a new frame.
        Use this to implement custom websocket protocols.

        Args:
            chunk (StreamChunk): The chunk that contains the frame
            frame (WebsocketFrame): The frame to handle

        Returns:
            WebsocketFrame: The frame to send to display. Header and Body concatenated.
        """
        return frame

    def handle_websocket_frames(self, chunk: StreamChunk) -> bytes:
        try:
            frames: List[bytes] = []
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

                websocket_frame = self.handle_websocket_frame(
                    chunk, websocket_frame)

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

    def decode_websocket_extensions(
            self,
            extensions_header: str) -> Dict[str, Dict[str, Union[bool, str]]]:
        extensions: Dict[str, Dict[str, Union[str, bool]]] = {}
        if extensions_header:
            raw_extensions = map(lambda s: s.strip().lower(),
                                 extensions_header.split(","))
            for extension in raw_extensions:
                extension, raw_params = extension.split(
                    ";", 1) if ";" in extension else (extension, '')
                params: Dict[str, Union[str, bool]] = {}
                raw_params = filter(
                    lambda p: len(p) != 0,
                    map(lambda p: p.strip(), raw_params.split(";")))
                for param in raw_params:
                    param = param.split("=", 1)
                    if len(param) == 1:
                        params[param[0]] = True
                    else:
                        if param[1].startswith('"') and param[1].endswith('"'):
                            param[1] = param[1][1:-1]
                        params[param[0]] = param[1]
                extensions[extension] = params
        return extensions

    def handle_raw_client_chunk(
            self, chunk: StreamChunk) -> Optional[List[StreamChunk]]:
        try:
            if self.switched_protocols:
                return [
                    StreamChunk(chunk.Direction,
                                self.handle_websocket_frames(chunk))
                ]
            return super().handle_raw_client_chunk(chunk)
        except Exception as ex:
            return [StreamChunk(chunk.Direction, str(ex).encode())]

    def handle_raw_server_chunk(
            self, chunk: StreamChunk) -> Optional[List[StreamChunk]]:
        try:
            if self.switched_protocols:
                return [
                    StreamChunk(chunk.Direction,
                                self.handle_websocket_frames(chunk))
                ]
            return super().handle_raw_server_chunk(chunk)
        except Exception as ex:
            return [StreamChunk(chunk.Direction, str(ex).encode())]

    def handle_http1_request(self, chunk: StreamChunk,
                             request: HTTPRequest) -> List[StreamChunk]:

        if request.headers.get(
                "Connection") == "Upgrade" and request.headers.get(
                    "Upgrade") == "websocket":
            websocket_key = request.headers.get("Sec-WebSocket-Key", None)
            if websocket_key is None:
                return [
                    StreamChunk(chunk.Direction, b"No websocket key found")
                ]
            self.websocket_key = websocket_key.encode()

        return super().handle_http1_request(chunk, request)

    def handle_http1_response(self, header: bytes, body: bytes,
                              chunk: StreamChunk,
                              response: HTTPResponse) -> List[StreamChunk]:
        try:
            if response.status == 101 and response.headers.get(
                    "Connection").lower(
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
                extensions_header = response.headers.get(
                    "Sec-WebSocket-Extensions")
                extensions = self.decode_websocket_extensions(
                    extensions_header)
                if "permessage-deflate" in extensions:
                    self.handle_permessage_deflate_extension(
                        extensions["permessage-deflate"])
                elif len(extensions) > 0:
                    self.log(f"Unsupported extensions: {extensions}")

                data = response.data
                if len(data) > 0:
                    data = self.handle_websocket_frames(
                        StreamChunk(chunk.Direction, data))

                return [
                    StreamChunk(chunk.Direction, header + b"\r\n\r\n" + data)
                ]

            return super().handle_http1_response(header, body, chunk, response)
        except Exception as ex:
            return [
                StreamChunk(chunk.Direction,
                            f"Unable to parse HTTP response: {ex}".encode())
            ]

    def handle_stream(self, stream: Stream) -> Result:
        self.websocket_key = None
        self.switched_protocols = False
        self.websocket_deflate = False
        self.websocket_deflate_decompressor = {}
        self.websocket_message_fragmented_frames = []
        return super().handle_stream(stream)


if __name__ == "__main__":
    WebsocketConverter().run()
