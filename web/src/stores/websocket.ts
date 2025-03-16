import { ClientConfig, ConverterStatistics, TagInfo } from "@/apiClient";
import { useRootStore } from ".";
import { useStreamStore } from "./stream";
import { useStreamsStore } from "./streams";
import {
  isConfigEvent,
  isConverterEvent,
  isEvent,
  isPcapStatsEvent,
  isTagEvent,
} from "./websocket.guard";

type EventTypes =
  | "configUpdated"
  | "converterCompleted"
  | "converterDeleted"
  | "converterAdded"
  | "converterRestarted"
  | "indexesMerged"
  | "pcapArrived"
  | "pcapProcessed"
  | "tagAdded"
  | "tagDeleted"
  | "tagUpdated"
  | "tagEvaluated";

/** @see {isEvent} ts-auto-guard:type-guard */
export type Event = {
  Type: EventTypes | string; // eslint-disable-line @typescript-eslint/no-redundant-type-constituents
};

/** @see {isTagEvent} ts-auto-guard:type-guard */
export type TagEvent = {
  Type: "tagAdded" | "tagDeleted" | "tagUpdated" | "tagEvaluated";
  Tag: TagInfo;
};

/** @see {isConverterEvent} ts-auto-guard:type-guard */
export type ConverterEvent = {
  Type:
    | "converterCompleted"
    | "converterDeleted"
    | "converterAdded"
    | "converterRestarted";
  Converter: ConverterStatistics;
};

export type PcapStats = {
  PcapCount: number;
  PacketCount: number;
  ImportJobCount: number;
  IndexCount: number;
  StreamCount: number;
  StreamRecordCount: number;
  PacketRecordCount: number;
};

/** @see {isPcapStatsEvent} ts-auto-guard:type-guard */
export type PcapStatsEvent = {
  Type: "pcapProcessed" | "indexesMerged";
  PcapStats: PcapStats;
};

/** @see {isConfigEvent} ts-auto-guard:type-guard */
export type ConfigEvent = {
  Type: "configUpdated";
  Config: ClientConfig;
};

export function setupWebsocket() {
  let reconnectTimeout = 125;
  const connect = () => {
    const l = window.location;
    const url = `ws${l.protocol.slice(4)}//${l.host}/ws`;
    const ws = new WebSocket(url);
    ws.onopen = () => {
      reconnectTimeout = 125;
      console.log("WebSocket connected");
    };
    ws.onerror = (err) => {
      console.error("WebSocket error:", err);
      ws.close();
    };
    ws.onclose = () => {
      console.log(
        `WebSocket closed, reconnecting in ${
          reconnectTimeout < 1000
            ? `1/${1000 / reconnectTimeout}`
            : reconnectTimeout / 1000.0
        }s`,
      );
      setTimeout(() => {
        if (reconnectTimeout < 10000) reconnectTimeout *= 2;
        connect();
      }, reconnectTimeout);
    };
    ws.onmessage = (event) => {
      if (typeof event.data !== "string") return;
      const store = useRootStore();
      const streamStore = useStreamStore();
      const streamsStore = useStreamsStore();
      const e = JSON.parse(event.data);
      if (!isEvent(e)) {
        console.error("Invalid event:", event.data);
        return;
      }
      switch (e.Type) {
        case "tagAdded":
          if (!isTagEvent(e)) {
            console.error("Invalid tag event:", e);
            return;
          }
          if (
            store.tags != null &&
            !store.tags.find((tag) => tag.Name == e.Tag.Name)
          ) {
            store.tags.push(e.Tag);
            store.tags.sort((a, b) => a.Name.localeCompare(b.Name));
          }
          break;
        case "tagDeleted":
          if (!isTagEvent(e)) {
            console.error("Invalid tag event:", e);
            return;
          }
          if (store.tags != null)
            store.tags = store.tags.filter((tag) => tag.Name != e.Tag.Name);
          if (streamStore.stream != null)
            streamStore.stream.Tags = streamStore.stream.Tags.filter(
              (tag) => tag !== e.Tag.Name,
            );
          if (streamsStore.result != null)
            streamsStore.result.Results = streamsStore.result.Results.map(
              (result) => {
                result.Tags = result.Tags.filter((tag) => tag !== e.Tag.Name);
                return result;
              },
            );
          break;
        case "tagUpdated":
        case "tagEvaluated":
          if (!isTagEvent(e)) {
            console.error("Invalid tag event:", e);
            return;
          }
          if (store.tags != null)
            store.tags = store.tags.map((t) =>
              t.Name == e.Tag.Name ? e.Tag : t,
            );
          break;
        case "converterAdded":
          if (!isConverterEvent(e)) {
            console.error("Invalid converter event:", e);
            return;
          }
          if (
            store.converters != null &&
            !store.converters.find((c) => c.Name == e.Converter.Name)
          ) {
            store.converters.push(e.Converter);
            store.converters.sort((a, b) => a.Name.localeCompare(b.Name));
          }
          break;
        case "converterDeleted":
          if (!isConverterEvent(e)) {
            console.error("Invalid converter event:", e);
            return;
          }
          if (store.converters != null)
            store.converters = store.converters.filter(
              (c) => c.Name != e.Converter.Name,
            );
          break;
        case "converterCompleted":
        case "converterRestarted":
          if (!isConverterEvent(e)) {
            console.error("Invalid converter event:", e);
            return;
          }
          if (store.converters != null) {
            store.converters = store.converters.map((c) =>
              c.Name == e.Converter.Name ? e.Converter : c,
            );
          }
          break;
        case "pcapProcessed":
        case "indexesMerged":
          if (!isPcapStatsEvent(e)) {
            console.error("Invalid pcap stats event:", e);
            return;
          }
          if (e.Type == "pcapProcessed") streamsStore.outdated = true;
          if (store.status != null) {
            store.status.PcapCount = e.PcapStats.PcapCount;
            store.status.PacketCount = e.PcapStats.PacketCount;
            store.status.ImportJobCount = e.PcapStats.ImportJobCount;
            store.status.IndexCount = e.PcapStats.IndexCount;
            store.status.StreamCount = e.PcapStats.StreamCount;
            store.status.StreamRecordCount = e.PcapStats.StreamRecordCount;
            store.status.PacketRecordCount = e.PcapStats.PacketRecordCount;
          }
          break;
        case "configUpdated":
          if (!isConfigEvent(e)) {
            console.error("Invalid config event:", e);
            return;
          }
          if (store.clientConfig != null) {
            store.clientConfig.AutoInsertLimitToQuery =
              e.Config.AutoInsertLimitToQuery;
          }
          break;
        default:
          console.log(`Unhandled event type: ${e.Type}`);
          break;
      }
    };
  };
  connect();
}
