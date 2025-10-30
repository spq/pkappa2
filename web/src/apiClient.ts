import axios from "axios";
import type { Base64, DateTimeString } from "@/types/common";
import {
  isConfig,
  isConvertersResponse,
  isGraphResponse,
  isMainStderr,
  isPcapOverIPResponse,
  isPcapsResponse,
  isProcessStderr,
  isSearchResponse,
  isStatistics,
  isStreamData,
  isTagsResponse,
  isWebhooks,
} from "./apiClient.guard";

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

/** @see {isError} ts-auto-guard:type-guard */
export type Error = {
  Error: string;
};

export type DataRegexes = {
  Client: string[] | null;
  Server: string[] | null;
};

/** @see {isSearchResult} ts-auto-guard:type-guard */
export type SearchResult = {
  Debug: string[];
  Results: Result[];
  Elapsed: number;
  Offset: number;
  /** If there are more results available to load */
  MoreResults: boolean;
  DataRegexes: DataRegexes;
};

/** @see {isSearchResponse} ts-auto-guard:type-guard */
export type SearchResponse = SearchResult | Error;

export type Data = {
  Direction: number;
  Content: Base64;
  Time?: DateTimeString;
  ContentType: string | undefined;
};

/** @see {isStreamData} ts-auto-guard:type-guard */
export type StreamData = {
  Stream: Stream;
  Data: Data[];
  Tags: string[];
  Converters: string[];
  ActiveConverter: string;
};

/** @see {isStatistics} ts-auto-guard:type-guard */
export type Statistics = {
  IndexCount: number;
  IndexLockCount: number;
  PcapCount: number;
  ImportJobCount: number;
  StreamCount: number;
  StreamRecordCount: number;
  PacketCount: number;
  PacketRecordCount: number;
  MergeJobRunning: boolean;
  TaggingJobRunning: boolean;
  ConverterJobRunning: boolean;
};

/** @see {isMainStderr} ts-auto-guard:type-guard */
export type MainStderr = string[];

/** @see {isConfig} ts-auto-guard:type-guard */
export type Config = {
  AutoInsertLimitToQuery: boolean;
};

export type PcapInfo = {
  Filename: string;
  Filesize: number;
  PacketTimestampMin: DateTimeString; // TODO: use moment
  PacketTimestampMax: DateTimeString;
  ParseTime: DateTimeString;
  PacketCount: number;
};

/** @see {isPcapsResponse} ts-auto-guard:type-guard */
export type PcapsResponse = PcapInfo[];

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

/** @see {isConvertersResponse} ts-auto-guard:type-guard */
export type ConvertersResponse = ConverterStatistics[];

/** @see {isProcessStderr} ts-auto-guard:type-guard */
export type ProcessStderr = {
  Pid: number;
  Stderr: string[];
};

export type PcapOverIPEndpoint = {
  Address: string;
  LastConnected: number;
  LastDisconnected: number;
  ReceivedPackets: number;
};

/** @see {isPcapOverIPResponse} ts-auto-guard:type-guard */
export type PcapOverIPResponse = PcapOverIPEndpoint[];

/** @see {isWebhooks} ts-auto-guard:type-guard */
export type Webhooks = string[];

export type TagInfo = {
  Name: string;
  Definition: string;
  Color: string;
  MatchingCount: number;
  UncertainCount: number;
  Referenced: boolean;
  Converters: string[];
};

/** @see {isTagsResponse} ts-auto-guard:type-guard */
export type TagsResponse = TagInfo[];

type GraphData = {
  Tags: string[];
  Data: number[][];
};

/** @see {isGraphResponse} ts-auto-guard:type-guard */
export type GraphResponse = {
  Min: DateTimeString; // TODO: use moment
  Max: DateTimeString;
  Delta: number;
  Aspects: string[];
  Data: GraphData[];
};

const APIClient = {
  async searchStreams(query: string, page: number) {
    return this.performGuarded(
      "post",
      "/search.json",
      isSearchResponse,
      query,
      {
        page,
      },
    );
  },
  async getStream(streamId: number, converter: string) {
    return this.performGuarded(
      "get",
      `/stream/${streamId}.json`,
      isStreamData,
      null,
      {
        converter,
      },
    );
  },
  async getStatus() {
    return this.performGuarded("get", `/status.json`, isStatistics);
  },
  async getMainStderr() {
    return this.performGuarded("get", `/stderr`, isMainStderr);
  },
  async getConfig() {
    return this.performGuarded("get", `/config`, isConfig);
  },
  async updateConfig(config: Config) {
    return this.perform("post", `/config`, JSON.stringify(config), undefined);
  },
  async getPcaps() {
    return this.performGuarded("get", `/pcaps.json`, isPcapsResponse);
  },
  async getPcapOverIPEndpoints() {
    return this.performGuarded("get", `/pcap-over-ip`, isPcapOverIPResponse);
  },
  async addPcapOverIPEndpoint(address: string) {
    return this.perform("put", `/pcap-over-ip`, null, { address });
  },
  async delPcapOverIPEndpoint(address: string) {
    return this.perform("delete", `/pcap-over-ip`, null, { address });
  },
  async getWebhooks() {
    return this.performGuarded("get", "/webhooks", isWebhooks);
  },
  async addWebhook(url: string) {
    return this.perform("put", "/webhooks", null, { url });
  },
  async delWebhook(url: string) {
    return this.perform("delete", "/webhooks", null, { url });
  },
  async getConverters() {
    return this.performGuarded("get", `/converters`, isConvertersResponse);
  },
  async getConverterStderrs(converter: string, pid: number) {
    return this.performGuarded(
      "get",
      `/converters/stderr/${converter}/${pid}`,
      isProcessStderr,
    );
  },
  async resetConverter(converter: string) {
    return this.perform("delete", `/converters/${converter}`);
  },
  async getTags() {
    return this.performGuarded("get", `/tags`, isTagsResponse);
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
  async changeTagDefinition(name: string, definition: string) {
    const params = new URLSearchParams();
    params.append("name", name);
    params.append("method", "change_query");
    params.append("query", definition);
    return this.perform("patch", `/tags`, null, params);
  },
  async changeTagName(name: string, newName: string) {
    const params = new URLSearchParams();
    params.append("name", name);
    params.append("method", "change_name");
    params.append("new_name", newName);
    return this.perform("patch", `/tags`, null, params);
  },
  async getGraph(
    delta: string,
    aspects: string[],
    tags: string[],
    query: string,
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
    return this.performGuarded(
      "get",
      "/graph.json",
      isGraphResponse,
      null,
      params,
    );
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

  _abort: null as null | (() => void),
  async perform(
    method: string,
    resource: string,
    data?: string | null,
    params?: object | URLSearchParams,
  ) {
    let signal: AbortSignal | undefined;
    if (resource == "/search.json" || resource == "/graph.json") {
      this._abort?.();
      const controller = new AbortController();
      this._abort = controller.abort.bind(controller);
      signal = controller.signal;
    }
    return client.request({
      method,
      url: resource,
      data,
      params,
      signal,
    });
  },
  async performGuarded<ResponseData>(
    method: string,
    resource: string,
    guard: (obj: unknown) => obj is ResponseData,
    data?: string | null,
    params?: object | URLSearchParams,
  ) {
    const response = await this.perform(method, resource, data, params);
    if (guard(response.data)) {
      return response.data;
    }
    throw new Error("Unexpected response, types mismatch");
  },
};

export default APIClient;
