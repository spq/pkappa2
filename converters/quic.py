#!/usr/bin/env python3
from aioquic._buffer import Buffer
from aioquic.quic.connection import dump_cid
from aioquic.quic.crypto import CryptoPair
from aioquic.quic.logger import QuicLoggerTrace
from aioquic.quic.packet import QuicPacketType, pull_quic_header
from scapy.layers.tls.all import TLS

from pkappa2lib import Direction, Pkappa2Converter, Result, Stream


class QUICConverter(Pkappa2Converter):
    def handle_stream(self, stream: Stream) -> Result:
        logger = QuicLoggerTrace(is_client=True, odcid=b"1234")
        result_data = []
        connection_id_length = 8
        for chunk in stream.coalesce_chunks_in_same_direction_iter():
            buf = Buffer(data=chunk.Content)
            while not buf.eof():
                start_off = buf.tell()
                output = ""
                try:
                    header = pull_quic_header(buf, host_cid_length=connection_id_length)

                    output = (
                        f"  version: {header.version}\n"
                        + f"  packet_type: {logger.packet_type(header.packet_type)} ({header.packet_type})\n"
                        + f"  packet length: {header.packet_length}\n"
                        + f"  destination_cid: {dump_cid(header.destination_cid)}\n"
                        + f"  source_cid: {dump_cid(header.source_cid)}\n"
                        + f"  token: {header.token!r}\n"
                        + f"  integrity_tag: {header.integrity_tag!r}\n"
                        + f"  supported versions: {header.supported_versions}\n"
                    )

                    encrypted_off = buf.tell() - start_off
                    end_off = header.packet_length + start_off
                    buf.seek(end_off)

                    if (
                        header.packet_type == QuicPacketType.INITIAL
                        and header.version is not None
                    ):
                        crypto = CryptoPair()
                        if chunk.Direction == Direction.CLIENTTOSERVER:
                            crypto.setup_initial(
                                header.destination_cid,
                                is_client=False,
                                version=header.version,
                            )
                        else:
                            crypto.setup_initial(
                                header.source_cid,
                                is_client=True,
                                version=header.version,
                            )
                        (
                            plain_header,
                            plain_payload,
                            packet_number,
                        ) = crypto.decrypt_packet(
                            chunk.Content[start_off:end_off], encrypted_off, 0
                        )
                        tls = TLS(plain_header + plain_payload)
                        output += (
                            f"  plain_header: {plain_header!r}\n"
                            + f"  plain_payload: \n{tls.show(dump=True)}\n"
                        )
                    else:
                        output += f"  encrypted: {chunk.Content[start_off:end_off]!r}\n"
                    result_data.append(chunk.derive(content=output.encode() + b"\n"))
                except Exception as ex:
                    result_data.append(
                        chunk.derive(
                            content=output.encode()
                            + str(ex).encode()
                            + b"\n"
                            + chunk.Content[start_off:]
                            + b"\n",
                        )
                    )
                    break

        return Result(result_data)


if __name__ == "__main__":
    QUICConverter().run()
