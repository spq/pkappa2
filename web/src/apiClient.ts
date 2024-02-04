import axios from "axios";
import type { Base64, DateTimeString } from "@/types/common";

const client = axios.create({
  baseURL: "/api/",
});

type SideInfo = {
  Host: string;
  Port: number;
  Bytes: number;
};

export type Stream = {
  ID: number;
  Protocol: string;
  Client: SideInfo;
  Server: SideInfo;
  FirstPacket: DateTimeString; // TODO: use moment
  LastPacket: DateTimeString;
  Index: string;
};

export type Result = {
  Stream: Stream;
  Tags: string[];
};

export type Error = {
  Error: string;
};

export type SearchResult = {
  Debug: string[];
  Results: Result[];
  Offset: number;
  MoreResults: boolean;
};

export type Data = {
  Direction: number;
  Content: Base64;
};

export type StreamData = {
  Stream: Stream;
  Data: Data[];
  Tags: string[];
  Converters: string[];
  ActiveConverter: string;
};

export type Statistics = {
  IndexCount: number;
  IndexLockCount: number;
  PcapCount: number;
  ImportJobCount: number;
  StreamCount: number;
  PacketCount: number;
  MergeJobRunning: boolean;
  TaggingJobRunning: boolean;
  ConverterJobRunning: boolean;
};

export type PcapInfo = {
  Filename: string;
  Filesize: number;
  PacketTimestampMin: DateTimeString; // TODO: use moment
  PacketTimestampMax: DateTimeString;
  ParseTime: DateTimeString;
  PacketCount: number;
};

export type ProcessStats = {
  Running: boolean;
  ExitCode: number;
  Pid: number;
  Errors: number;
};

export type ConverterStatistics = {
  Name: string;
  CachedStreamCount: number;
  Processes: ProcessStats[];
};

export type ProcessStderr = {
  Pid: number;
  Stderr: string[];
};

export type TagInfo = {
  Name: string;
  Definition: string;
  Color: string;
  MatchingCount: number;
  UncertainCount: number;
  Referenced: boolean;
  Converters: string[];
};

type GraphData = {
  Tags: string[];
  Data: number[][];
};

export type GraphResponse = {
  Min: DateTimeString; // TODO: use moment
  Max: DateTimeString;
  Delta: number;
  Aspects: string[];
  Data: GraphData[];
};

// TODO: Verify response types!
const APIClient = {
  async searchStreams(query: string, page: number) {
    return this.perform<SearchResult | Error>("post", "/search.json", query, {
      page,
    });
  },
  async getStream(streamId: number, converter: string) {
    return this.perform<StreamData>("get", `/stream/${streamId}.json`, null, {
      converter,
    });
  },
  async getStatus() {
    return this.perform<Statistics>("get", `/status.json`);
  },
  async getPcaps() {
    return this.perform<PcapInfo[]>("get", `/pcaps.json`);
  },
  async getConverters() {
    return this.perform<ConverterStatistics[]>("get", `/converters`);
  },
  async getConverterStderrs(converter: string, pid: number) {
    return this.perform<ProcessStderr>(
      "get",
      `/converters/stderr/${converter}/${pid}`
    );
  },
  async resetConverter(converter: string) {
    return this.perform("delete", `/converters/${converter}`);
  },
  async getTags() {
    return this.perform<TagInfo[]>("get", `/tags`);
  },
  async addTag(name: string, query: string, color: string) {
    return this.perform("put", `/tags`, query, { name, color });
  },
  async delTag(name: string) {
    return this.perform("delete", `/tags`, null, { name });
  },
  async changeTagColor(name: string, color: string) {
    const params = new URLSearchParams();
    params.append("name", name);
    params.append("method", "change_color");
    params.append("color", color);
    return this.perform("patch", `/tags`, null, params);
  },
  async getGraph(
    delta: string,
    aspects: string[],
    tags: string[],
    query: string
  ) {
    const params = new URLSearchParams();
    params.append("delta", delta);
    for (const a of aspects) {
      params.append("aspect", a);
    }
    for (const t of tags) {
      params.append("tag", t);
    }
    if (query) {
      params.append("query", query);
    }
    return this.perform<GraphResponse>("get", "/graph.json", null, params);
  },
  async markTagNew(name: string, streams: number[], color: string) {
    if (streams.length == 0) streams = [-1];
    return this.addTag(name, `id:${streams.join(",")}`, color);
  },
  async converterTagSet(tagName: string, converters: string[]) {
    const params = new URLSearchParams();
    params.append("name", tagName);
    params.append("method", "converter_set");
    for (const c of converters) {
      params.append("converters", c);
    }
    return this.perform("patch", `/tags`, null, params);
  },
  async markTagAdd(name: string, streams: number[]) {
    const params = new URLSearchParams();
    params.append("name", name);
    params.append("method", "mark_add");
    for (const s of streams) {
      params.append("stream", s.toString());
    }
    return this.perform("patch", `/tags`, null, params);
  },
  async markTagDel(name: string, streams: number[]) {
    const params = new URLSearchParams();
    params.append("name", name);
    params.append("method", "mark_del");
    for (const s of streams) {
      params.append("stream", s.toString());
    }
    return this.perform("patch", `/tags`, null, params);
  },

  async perform<ResponseData = unknown>(
    method: string,
    resource: string,
    data?: string | null,
    params?: object | URLSearchParams
  ) {
    return client
      .request<ResponseData>({
        method,
        url: resource,
        data,
        params,
      })
      .then((req) => {
        return req.data;
      });
  },
};

export default APIClient;
