#!/usr/bin/env python3
from scapy.layers.tls.all import TLS

from pkappa2lib import Pkappa2Converter, Result, Stream, StreamChunk


class TLSConverter(Pkappa2Converter):
    def handle_stream(self, stream: Stream) -> Result:
        result_data = []
        for chunk in stream.Chunks:
            try:
                tls = TLS(chunk.Content)
                result_data.append(
                    StreamChunk(chunk.Direction, tls.show(dump=True).encode())
                )
            except Exception as ex:
                result_data.append(
                    StreamChunk(
                        chunk.Direction, str(ex).encode() + b"\n" + chunk.Content
                    )
                )

        return Result(result_data)


if __name__ == "__main__":
    TLSConverter().run()
