from dataclasses import dataclass
from enum import Enum
from typing import List
import json
import sys
import base64


class Protocol(Enum):
    TCP = 0
    UDP = 1

    @staticmethod
    def from_json(json_value):
        if json_value == "TCP":
            return Protocol.TCP
        elif json_value == "UDP":
            return Protocol.UDP
        else:
            raise ValueError(f"Unknown protocol: {json_value}")


@dataclass
class StreamMetadata:
    StreamID: int
    ClientHost: str
    ClientPort: int
    ServerHost: str
    ServerPort: int
    Protocol: Protocol


class ConverterDecoder(json.JSONDecoder):

    def __init__(self, *args, **kwargs):
        super().__init__(object_hook=self.object_hook, *args, **kwargs)

    @staticmethod
    def object_hook(obj):
        if "Protocol" in obj:
            obj["Protocol"] = Protocol.from_json(obj["Protocol"])
        if "Direction" in obj:
            obj["Direction"] = Direction.from_json(obj["Direction"])
        if "Content" in obj:
            obj["Content"] = base64.b64decode(obj["Content"])
        return obj


class ConverterEncoder(json.JSONEncoder):

    def default(self, obj):
        if isinstance(obj, StreamChunk):
            return {
                "Content": base64.b64encode(obj.Content).decode(),
                "Direction": obj.Direction.to_json()
            }

        else:
            return super().default(obj)


class Direction(Enum):
    CLIENTTOSERVER = 0
    SERVERTOCLIENT = 1

    @staticmethod
    def from_json(json_value):
        if json_value == "client-to-server":
            return Direction.CLIENTTOSERVER
        elif json_value == "server-to-client":
            return Direction.SERVERTOCLIENT
        else:
            raise ValueError(f"Unknown direction: {json_value}")

    def to_json(self):
        if self == Direction.CLIENTTOSERVER:
            return "client-to-server"
        elif self == Direction.SERVERTOCLIENT:
            return "server-to-client"
        else:
            raise ValueError(f"Unknown direction: {self}")


@dataclass
class StreamChunk:
    Direction: Direction
    Content: bytes


@dataclass
class Stream:
    Metadata: StreamMetadata
    Chunks: List[StreamChunk]


@dataclass
class Result:
    Chunks: List[StreamChunk]


class Pkappa2Converter:
    """
    Base class for pkappa2 converters.

    Converters are expected to be implemented as a class that inherits from this
    class and implements the handle_stream method. The handle_stream method
    is called for each stream that is passed to the converter. The converter is
    expected to return a Result object that contains the data that should be
    displayed in the UI.
    """

    def run(self):
        """
        Run the converter.
        
        This method goes into an endless loop that parses the input from
        pkappa2 and calls the handle_stream method for each stream. The
        result of the handle_stream method is then written to stdout.
        """
        while True:
            try:
                metadata_json = json.loads(sys.stdin.buffer.readline().decode(),
                                        cls=ConverterDecoder)
                metadata = StreamMetadata(**metadata_json)
                stream_chunks = []
                while True:
                    line = sys.stdin.buffer.readline().strip()
                    if not line:
                        break
                    chunk = json.loads(line.decode(), cls=ConverterDecoder)
                    stream_chunks.append(StreamChunk(**chunk))
                stream = Stream(metadata, stream_chunks)
                result = self.handle_stream(stream)
                for chunk in result.Chunks:
                    json.dump(chunk, sys.stdout, cls=ConverterEncoder)
                    print("")
                print("")
                print("{}", flush=True)
            except KeyboardInterrupt:
                break

    def handle_stream(self, stream: Stream) -> Result:
        """
        Transform the data of a stream and return the changed stream.
        The stream contains metadata of the source and target and a list of
        chunks of data. Each chunk contains the direction of the data and the
        data itself. The data is a byte array.

        This method is called for each stream that is passed to the converter.

        Args:
            stream: The stream to transform.

        Returns:
            A Result object that contains the data that should be displayed
            in the UI.
        """
        raise NotImplementedError