#!/usr/bin/env python3
from pkappa2lib import *


class Base64DecodeConverter(Pkappa2Converter):

    def handle_stream(self, stream: Stream) -> Result:
        result_data = []
        for chunk in stream.Chunks:
            try:
                data = base64.b64decode(chunk.Content)
            except Exception as ex:
                data = f"Unable to decode: {ex}".encode()
            result_data.append(StreamChunk(chunk.Direction, data))
        return Result(result_data)


if __name__ == "__main__":
    Base64DecodeConverter().run()
