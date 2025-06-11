#!/usr/bin/env python3
import re
from io import BytesIO

from protobuf_inspector.types import StandardParser

from pkappa2lib import Pkappa2Converter, Result, Stream


class ProtobufConverter(Pkappa2Converter):
    def __init__(self):
        super().__init__()
        self._ansi_escape = re.compile(r"\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])")

    def handle_stream(self, stream: Stream) -> Result:
        result_data = []
        for chunk in stream.coalesce_chunks_in_same_direction_iter():
            try:
                parser = StandardParser()
                frame_data = BytesIO(chunk.Content)
                protobuf_message = parser.parse_message(frame_data, "message")
                result_data.append(
                    chunk.derive(
                        content=self._ansi_escape.sub("", protobuf_message).encode(),
                    )
                )
            except Exception as ex:
                result_data.append(
                    chunk.derive(
                        content=b"Protobuf ERROR: "
                        + str(ex).encode()
                        + b"\n"
                        + chunk.Content,
                    )
                )
        return Result(result_data)


if __name__ == "__main__":
    ProtobufConverter().run()
