#!/usr/bin/env python3
from scapy.layers.tls.all import (
    TLS,
    TLSApplicationData,
    Cert,
    PrivKey,
    tlsSession,
    load_nss_keys,
)

from pkappa2lib import Pkappa2Converter, Result, Stream, StreamChunk
from pathlib import Path

# https://github.com/secdev/scapy/blob/5160430bd16c6084d5aef2a10e47dc0455aace40/doc/notebooks/tls/notebook3_tls_compromised.ipynb
# Place your key.pem in the converters/tls folder
# Perfect Forward Secrecy (PFS) has to be disabled.
key_path = Path(__file__).parent / "tls" / "key.pem"
# Optionally place your server cert.pem in the converters/tls folder
cert_path = Path(__file__).parent / "tls" / "cert.pem"

# if PFS is enabled, you can use the nss_keylog.txt file to decrypt the traffic
# use the `-keylogfile` option to enable it in openssl
#    e.g.:   openssl s_client -connect localhost:443 -keylogfile nss_keylog.txt
# or set the environment variable `SSLKEYLOGFILE` to the path of the file
#   e.g.:   export SSLKEYLOGFILE=/path/to/converters/tls/nss_keylog.txt
nss_keylog_path = Path(__file__).parent / "tls" / "nss_keylog.txt"


class TLSConverter(Pkappa2Converter):
    def handle_stream(self, stream: Stream) -> Result:
        tls_session = tlsSession()
        tls_session.sport = stream.Metadata.ClientPort
        tls_session.dport = stream.Metadata.ServerPort
        tls_session.ipsrc = stream.Metadata.ClientHost
        tls_session.ipdst = stream.Metadata.ServerHost
        if key_path.exists():
            key = PrivKey(key_path)
            tls_session.server_rsa_key = key
        if cert_path.exists():
            cert = Cert(cert_path)
            tls_session.server_certs = [cert]
        if nss_keylog_path.exists():
            tls_session.nss_keys = load_nss_keys(nss_keylog_path)

        result_data = []
        for chunk in stream.Chunks:
            try:
                tls = TLS(chunk.Content, tls_session=tls_session)
                tls_session = tls.tls_session.mirror()
                if TLSApplicationData in tls:
                    result_data.append(
                        StreamChunk(
                            chunk.Direction,
                            tls[TLSApplicationData].data,
                        )
                    )
                else:
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
