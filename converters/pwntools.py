#!/usr/bin/env python3
from pkappa2lib import (
    ATTACK_INFO_URL,
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
    attackinfo_part: bytes


class PwntoolsRemoteConverter(Pkappa2Converter):
    def handle_stream(self, stream: Stream) -> Result:
        attack_infos = self.get_attackinfo(stream)
        uses_attackinfo = False
        chunks: list[Chunk] = []
        output = ""
        for i, chunk in enumerate(stream.Chunks):
            data_recvuntil = chunk.Content
            attackinfo_part = b""
            # recvuntil after the last newline
            # b'bla\n...\n -> b'...\n'
            if chunk.Direction == Direction.SERVERTOCLIENT:
                no_end = data_recvuntil.rstrip(b"\n")
                end_newlines = b"\n" * (len(data_recvuntil) - len(no_end))
                newline_idx = no_end.rfind(b"\n")
                if newline_idx != -1:
                    no_end = no_end[newline_idx + 1 :]
                data_recvuntil = no_end + end_newlines
            else:  # Direction.CLIENTTOSERVER
                # See if the client sent something from the attackinfo
                for attackinfo in attack_infos:
                    attackinfo_b = attackinfo.encode()
                    if attackinfo_b in chunk.Content:
                        attackinfo_part = attackinfo_b
                        uses_attackinfo = True
                        break

            # truncate long data arbitrarily
            data_recvuntil = data_recvuntil[-RECEIVE_UNTIL_MAX:]
            chunks.append(
                Chunk(
                    chunk.Content,
                    data_recvuntil,
                    chunk.Content[-1:] == b"\n",
                    chunk.Direction,
                    attackinfo_part,
                )
            )

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
                if pchunk.attackinfo_part:
                    # split sent data into chunks seperated by attackinfo
                    # and insert attack_info variable in between
                    parts = data.split(pchunk.attackinfo_part)
                    attackinfo_out = ""
                    for idx, part in enumerate(parts):
                        if attackinfo_out:
                            if part:
                                attackinfo_out += f" + {part!r}"
                            if idx < len(parts) - 1:
                                attackinfo_out += " + attack_info.encode()"
                        else:
                            if part:
                                attackinfo_out += f"{part!r} + attack_info.encode()"
                            else:
                                attackinfo_out += "attack_info.encode()"
                    output += f"io.{fn}({after_data}{attackinfo_out})\n"
                else:
                    output += f"io.{fn}({after_data}{data!r})\n"
                after_data = ""

        typ = ""
        if stream.Metadata.Protocol == Protocol.UDP:
            typ = ', typ = "udp"'

        # TODO: look for data that is received from the server and sent back later
        #       and receive that data into variable and use it later instead of hardcoding the value
        #       from the traffic
        if uses_attackinfo:
            indented_output = "\n".join(
                " " * 4 + line for line in output.split("\n") if line
            )
            output = f"""#!/usr/bin/env python3
from pwn import *
import sys
import requests

context.timeout = 10

# IP = {stream.Metadata.ServerHost!r}
IP = sys.argv[1]

def get_attack_info() -> list[str]:
    r = requests.get(
        "{ATTACK_INFO_URL}",
        params={{
            "service_ip": {stream.Metadata.ServerHost!r},
            "service_port": {stream.Metadata.ServerPort},
            "target_ip": IP,
        }},
        timeout=2,
    )
    r.raise_for_status()
    return r.json()

for attack_info in get_attack_info():
    # Generated from stream {stream.Metadata.StreamID}
    io = remote(IP, {stream.Metadata.ServerPort}{typ})
{indented_output}
    io.stream()
"""
        else:
            # No attackinfo involved.
            output = f"""#!/usr/bin/env python3
from pwn import *
import sys

context.timeout = 10

# IP = {stream.Metadata.ServerHost!r}
IP = sys.argv[1]

# Generated from stream {stream.Metadata.StreamID}
io = remote(IP, {stream.Metadata.ServerPort}{typ})
{output}
io.stream()
"""

        return Result([StreamChunk(Direction.CLIENTTOSERVER, output.encode())])


if __name__ == "__main__":
    PwntoolsRemoteConverter().run()
