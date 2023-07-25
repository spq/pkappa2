#!/usr/bin/env python3
from pkappa2lib import Pkappa2Converter, Result, Stream, StreamChunk
from dnslib import DNSRecord


class DNSConverter(Pkappa2Converter):

    def handle_stream(self, stream: Stream) -> Result:
        result_data = []
        for chunk in stream.Chunks:
            try:
                dns = DNSRecord.parse(chunk.Content)
                result_data.append(
                    StreamChunk(chunk.Direction,
                                str(dns).encode() + b'\n'))
            except Exception as ex:
                result_data.append(
                    StreamChunk(chunk.Direction,
                                str(ex).encode() + b'\n' + chunk.Content))
        return Result(result_data)


if __name__ == "__main__":
    DNSConverter().run()
