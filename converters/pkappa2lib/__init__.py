import base64
import datetime
import json
import sys
from dataclasses import dataclass
from enum import Enum
from typing import List


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
        super().__init__(object_hook=self.converter_object_hook, *args, **kwargs)

    @staticmethod
    def converter_object_hook(obj):
        if "Protocol" in obj:
            obj["Protocol"] = Protocol.from_json(obj["Protocol"])
        if "Direction" in obj:
            obj["Direction"] = Direction.from_json(obj["Direction"])
        if "Content" in obj:
            obj["Content"] = base64.b64decode(obj["Content"])
        if "Time" in obj:
            # Make sure there are 6 digits in the microseconds part
            if "." in obj["Time"]:
                time_parts = obj["Time"].split(".")
                if len(time_parts) == 2:
                    time_parts[1] = time_parts[1].ljust(6, "0")
                obj["Time"] = ".".join(time_parts)
            obj["Time"] = datetime.datetime.strptime(
                obj["Time"], "%Y-%m-%dT%H:%M:%S.%f"
            )

        return obj


class ConverterEncoder(json.JSONEncoder):
    def default(self, o):
        if isinstance(o, StreamChunk):
            return {
                "Content": base64.b64encode(o.Content).decode(),
                "Direction": o.Direction.to_json(),
                "Time": o.Time.strftime("%Y-%m-%dT%H:%M:%S.%f"),
            }

        else:
            return super().default(o)


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
    Time: datetime.datetime

    def derive(
        self,
        *,
        direction: Direction | None = None,
        content: bytes | None = None,
        time: datetime.datetime | None = None,
    ) -> "StreamChunk":
        """
        Derive a new StreamChunk with the given parameters.
        If a parameter is not provided, the current value is used.
        """
        return StreamChunk(
            Direction=direction if direction is not None else self.Direction,
            Content=content if content is not None else self.Content,
            Time=time if time is not None else self.Time,
        )


@dataclass
class Stream:
    Metadata: StreamMetadata
    Chunks: List[StreamChunk]

    def coalesce_chunks_in_same_direction_iter(self):
        """
        Coalesce chunks in the same direction into a single chunk.
        This method yields chunks where consecutive chunks in the same direction
        are combined into a single chunk.

        There can be multiple chunks in the same direction with differing Time.
        This method will yield a new chunk for each change in direction with the
        Time of the first chunk in that direction.
        """
        if not self.Chunks:
            return
        current_chunk = self.Chunks[0]
        for chunk in self.Chunks[1:]:
            if chunk.Direction == current_chunk.Direction:
                current_chunk.Content += chunk.Content
            else:
                yield current_chunk
                current_chunk = chunk

        yield current_chunk


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

    current_stream_id: int

    def log(self, message: str):
        """
        Log a message to stderr.

        This method can be used to log messages to the UI. The message will be
        displayed in the stderr tab of the UI.

        Can be used for debugging.
        """
        now = datetime.datetime.now().strftime("%d.%b %Y %H:%M:%S")
        print(
            f"{now} (stream: {self.current_stream_id}): {message}",
            flush=True,
            file=sys.stderr,
        )

    def run(self):
        """
        Run the converter.

        This method goes into an endless loop that parses the input from
        pkappa2 and calls the handle_stream method for each stream. The
        result of the handle_stream method is then written to stdout.
        """
        self.current_stream_id = -1
        while True:
            try:
                metadata_json = json.loads(
                    sys.stdin.buffer.readline().decode(), cls=ConverterDecoder
                )
                metadata = StreamMetadata(**metadata_json)
                self.current_stream_id = metadata.StreamID
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
