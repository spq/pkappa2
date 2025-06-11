#!/usr/bin/env python3
from datetime import datetime
from pkappa2lib import (
    Direction,
    Pkappa2Converter,
    Protocol,
    Result,
    Stream,
    StreamChunk,
)
from dataclasses import dataclass


# Maximum number of bytes to receive until
RECEIVE_UNTIL_MAX = 40


@dataclass
class Chunk:
    data: bytes
    data_recvuntil: bytes
    isline: bool
    direction: Direction


class PwntoolsRemoteConverter(Pkappa2Converter):
    def handle_stream(self, stream: Stream) -> Result:
        typ = ""
        if stream.Metadata.Protocol == Protocol.UDP:
            typ = ', typ = "udp"'
        output = f"""#!/usr/bin/env python3
from pwn import *
import sys

# Generated from stream {stream.Metadata.StreamID}
# io = remote({stream.Metadata.ServerHost!r}, {stream.Metadata.ServerPort}{typ})
io = remote(sys.argv[1], {stream.Metadata.ServerPort}{typ})
"""
        chunks: list[Chunk] = []
        for chunk in stream.coalesce_chunks_in_same_direction_iter():
            data_recvuntil = chunk.Content
            # recvuntil after the last newline
            # b'bla\n...\n -> b'...\n'
            if chunk.Direction == Direction.SERVERTOCLIENT:
                no_end = data_recvuntil.rstrip(b"\n")
                end_newlines = b"\n" * (len(data_recvuntil) - len(no_end))
                newline_idx = no_end.rfind(b"\n")
                if newline_idx != -1:
                    no_end = no_end[newline_idx + 1 :]
                data_recvuntil = no_end + end_newlines

            # truncate long data arbitrarily
            data_recvuntil = data_recvuntil[-RECEIVE_UNTIL_MAX:]
            chunks.append(
                Chunk(
                    chunk.Content,
                    data_recvuntil,
                    chunk.Content[-1:] == b"\n",
                    chunk.Direction,
                )
            )

        # TODO: look for data that is received from the server and sent back later
        #       and receive that data into variable and use it later instead of hardcoding the value
        #       from the traffic

        # Collapse recvuntil + send into a single sendafter (line)
        after_data = ""
        for i, pchunk in enumerate(chunks):
            if pchunk.direction == Direction.SERVERTOCLIENT:
                if (
                    i + 1 < len(chunks)
                    and chunks[i + 1].direction == Direction.CLIENTTOSERVER
                ):
                    after_data = f"{chunks[i].data_recvuntil!r}, "
            else:
                data = pchunk.data[:-1] if pchunk.isline else pchunk.data
                fn = "sendline" if pchunk.isline else "send"
                fn += "after" if after_data else ""
                output += f"io.{fn}({after_data}{data!r})\n"
                after_data = ""

        output += "io.stream()\n"
        return Result(
            [
                StreamChunk(
                    Direction.CLIENTTOSERVER,
                    output.encode(),
                    stream.Chunks[0].Time if stream.Chunks else datetime.now(),
                )
            ]
        )


if __name__ == "__main__":
    PwntoolsRemoteConverter().run()
