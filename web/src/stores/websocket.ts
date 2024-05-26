import { TagInfo } from "@/apiClient";
import { useRootStore } from ".";

type Event = {
  Type: string;
  Tag: TagInfo | null;
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
      const store = useRootStore();
      const e: Event = JSON.parse(event.data as string) as Event;
      switch (e.Type) {
        case "tagAdded":
          if (store.tags != null) store.tags.push(e.Tag!);
          break;
        case "tagDeleted":
          if (store.tags != null)
            store.tags = store.tags.filter((tag) => tag.Name != e.Tag?.Name);
          break;
        case "tagUpdated":
        case "tagEvaluated":
          if (store.tags != null)
            store.tags = store.tags.map((t) =>
              t.Name == e.Tag!.Name ? e.Tag! : t
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
