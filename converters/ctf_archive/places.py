#!/usr/bin/env python3
# Challenge converter of "places" challenge in HITB SECCONF CTF 2023.
# https://2023.ctf.hitb.org/hitb-ctf-phuket-2023/
import base64
import json
from dataclasses import dataclass
from struct import unpack
from typing import List
from uuid import UUID

from Crypto.Cipher import AES

from http_gzip import HTTPConverter, HTTPRequest, HTTPResponse
from pkappa2lib import Result, Stream, StreamChunk

encryption_key = bytes.fromhex("b9fac3345069c4cb6e6e2ea47e17b0eb")


def decrypt(data):
    aes = AES.new(encryption_key, AES.MODE_ECB)
    return aes.decrypt(data)


@dataclass
class Place:
    userId: UUID
    lat: float
    long: float


def PlaceIdFromString(data):
    place_raw = bytes.fromhex(data)
    place_bytes = decrypt(place_raw)
    if len(place_bytes) != 32:
        return None
    return Place(UUID(bytes=place_bytes[0:16]), *unpack("<dd", place_bytes[16:]))


# print(
#     PlaceIdFromString(
#         "338cde0ca0df42379e4a88cb2c681b66cf49ce8b6b997d1b32c5259cdd516ec7"
#     )
# )


class PlacesConverter(HTTPConverter):
    is_place_request: bool

    def handle_http1_request(
        self, chunk: StreamChunk, request: HTTPRequest
    ) -> List[StreamChunk]:
        try:
            headers, body = chunk.Content.split(b"\r\n\r\n", 1)
            cookies = request.headers.get_all("Cookie")
            jwt_decoded = []
            if cookies:
                for cookie in cookies:
                    cookie_name, cookie_value = cookie.split("=", 1)
                    if cookie_name == "auth":
                        jwt = cookie_value.split(".")[1]
                        jwt += "=" * (-len(jwt) % 4)
                        jwt_decoded.append(base64.b64decode(jwt))

            if jwt_decoded:
                jwt_decoded_string = b"\n".join(jwt_decoded)
                chunk.Content = (
                    headers
                    + b"\n\n### JWT payload: "
                    + jwt_decoded_string
                    + b" ###\r\n\r\n"
                    + body
                )
            if request.command == "GET":
                if request.path.startswith("/api/get/place/"):
                    placeid = request.path[len("/api/get/place/") :]
                    place = PlaceIdFromString(placeid)
                    prefix = f"### {str(place)} ###\n".encode()
                    return [chunk.derive(content=prefix + chunk.Content)]
                elif request.path.startswith("/api/auth"):
                    self.is_place_request = True

            elif request.command == "POST":
                if request.path == "/api/route":
                    _, body = chunk.Content.split(b"\r\n\r\n", 1)
                    route = json.loads(body)
                    decoded = [str(PlaceIdFromString(placeid)) for placeid in route]
                    return [
                        chunk.derive(
                            content=chunk.Content
                            + b"\n\n### Places decoded ###\n"
                            + "\n".join(decoded).encode(),
                        )
                    ]
            elif request.command == "PUT":
                if request.path.startswith("/api/put/place"):
                    self.is_place_request = True
                    placeid = request.path[len("/api/put/place/") :]
                    place = PlaceIdFromString(placeid)
                    prefix = f"### {str(place)} ###\n".encode()
                    return [chunk.derive(content=prefix + chunk.Content)]

        except Exception as ex:
            self.log(str(ex))
            return [chunk.derive(content=str(ex).encode() + chunk.Content)]
        return super().handle_http1_request(chunk, request)

    def handle_http1_response(
        self, header: bytes, body: bytes, chunk: StreamChunk, response: HTTPResponse
    ) -> List[StreamChunk]:
        try:
            if self.is_place_request:
                self.is_place_request = False
                placeid = response.data.decode()
                place = PlaceIdFromString(placeid)
                data = (
                    header
                    + b"\r\n\r\n"
                    + response.data
                    + b"\n### "
                    + str(place).encode()
                )
                return [chunk.derive(content=data)]
        except Exception as ex:
            self.log(str(ex))
        return super().handle_http1_response(header, body, chunk, response)

    def handle_stream(self, stream: Stream) -> Result:
        self.is_place_request = False
        return super().handle_stream(stream)


if __name__ == "__main__":
    PlacesConverter().run()
