import { TagInfo } from "@/apiClient";
import { useRootStore } from ".";
import { useStreamStore } from "./stream";
import { useStreamsStore } from "./streams";
import { isEvent, isTagEvent } from "./websocket.guard";

type EventTypes =
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
  Type: EventTypes | string;
};

/** @see {isTagEvent} ts-auto-guard:type-guard */
export type TagEvent = {
  Type: "tagAdded" | "tagDeleted" | "tagUpdated" | "tagEvaluated";
  Tag: TagInfo;
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
        }s`
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
      // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
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
          )
            store.tags.push(e.Tag);
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
              (tag) => tag !== e.Tag.Name
            );
          if (streamsStore.result != null)
            streamsStore.result.Results = streamsStore.result.Results.map(
              (result) => {
                result.Tags = result.Tags.filter((tag) => tag !== e.Tag.Name);
                return result;
              }
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
              t.Name == e.Tag.Name ? e.Tag : t
            );
          break;
        default:
          console.log(`Unhandled event type: ${e.Type}`);
          break;
      }
    };
  };
  connect();
}
