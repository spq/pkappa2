#!/usr/bin/env python3
from dnslib import DNSRecord

from pkappa2lib import Pkappa2Converter, Result, Stream


class DNSConverter(Pkappa2Converter):
    def handle_stream(self, stream: Stream) -> Result:
        result_data = []
        for chunk in stream.coalesce_chunks_in_same_direction_iter():
            try:
                dns = DNSRecord.parse(chunk.Content)
                result_data.append(chunk.derive(content=str(dns).encode() + b"\n"))
            except Exception as ex:
                result_data.append(
                    chunk.derive(content=str(ex).encode() + b"\n" + chunk.Content)
                )
        return Result(result_data)


if __name__ == "__main__":
    DNSConverter().run()
