#!/usr/bin/env python3
import base64
import binascii
import re

from pkappa2lib import Pkappa2Converter, Result, Stream


class Base64DecodeConverter(Pkappa2Converter):
    def __init__(self):
        super().__init__()
        self._pattern = re.compile(
            rb"([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{2}==)?"
        )

    def decode_possible_base64(self, data: bytes) -> bytes:
        content = b""
        pos = 0
        for match in self._pattern.finditer(data):
            content += data[pos : match.start()]

            # Some heuristics to determine if the data is base64 encoded
            chunk = match.group(0).decode()
            uppercase = len(list(filter(lambda c: c.isupper(), chunk)))
            lowercase = len(list(filter(lambda c: c.islower(), chunk)))
            digits = len(list(filter(lambda c: c.isdigit(), chunk)))

            if uppercase > 0 and lowercase > 0 and digits > 0:
                try:
                    content += base64.b64decode(match.group(0))
                except binascii.Error:
                    content += data[match.start() : match.end()]
            else:
                content += data[match.start() : match.end()]
            pos = match.end()
        content += data[pos:]
        return content

    def handle_stream(self, stream: Stream) -> Result:
        result_data = []
        for chunk in stream.coalesce_chunks_in_same_direction_iter():
            content = self.decode_possible_base64(chunk.Content)
            result_data.append(chunk.derive(content=content))
        return Result(result_data)


if __name__ == "__main__":
    Base64DecodeConverter().run()
