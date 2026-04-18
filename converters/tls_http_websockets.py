#!/usr/bin/env python3
from tls import TLSConverter
from websockets import WebsocketConverter
from pkappa2lib import Result, Stream, StreamChunk


class DecryptedHTTPSConverter(TLSConverter):
    def __init__(self):
        super().__init__()
        self.websocket_converter = WebsocketConverter()
        self.decrypted_chunks = []

    def handle_decrypted_chunk(self, chunk: StreamChunk) -> None:
        self.decrypted_chunks.append(chunk)

    def handle_stream(self, stream: Stream) -> Result:
        self.decrypted_chunks = []
        super().handle_stream(stream)
        self.websocket_converter.current_stream_id = self.current_stream_id
        return self.websocket_converter.handle_stream(
            Stream(
                Metadata=stream.Metadata,
                Chunks=self.decrypted_chunks,
            )
        )


if __name__ == "__main__":
    DecryptedHTTPSConverter().run()
