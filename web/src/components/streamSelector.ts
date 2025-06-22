import ListenerBag from "./listenerBag";
import { useStreamStore } from "@/stores/stream";
import { Ref } from "vue";
import type { ComponentPublicInstance } from "vue";
import { Data } from "@/apiClient";
import { decodeChunkContent, escape } from "@/lib/utils";

const listenerBag = new ListenerBag();

type ThisProxy = {
  streamData: ComponentPublicInstance | null;
  presentation: Ref<string>;
  urlDecode: Ref<boolean>;
  selectionData: Ref<string>;
  selectionQuery: Ref<string>;
};

export function registerSelectionListener(streamInstance: ThisProxy) {
  listenerBag.addListener(
    document,
    "selectionchange",
    onSelectionChange.bind(streamInstance),
  );
}

export function destroySelectionListener() {
  listenerBag.clear();
}

function getFromDataSet(outerBound: Node, container: Node, data: string) {
  const node = getDataSetContainer(outerBound, container, data);
  if (node == null) return null;
  return node.dataset[data] ?? null;
}

/**
 * @param {Node} outerBound The outer bound of the search
 * @param {Node} container The current node to search
 * @param {string} data The data attribute to search for
 * @returns {HTMLElement|null} The closest parent with the given data attribute or null if none is found
 **/
function getDataSetContainer(outerBound: Node, container: Node, data: string) {
  let currentNode: Node | null = container;
  // Ignore non-HTMLElement nodes and look for ones that have a dataset with our data attribute
  if (!outerBound.contains(currentNode)) {
    return null;
  }
  while (
    !(currentNode instanceof HTMLElement) ||
    currentNode?.dataset?.[data] == null
  ) {
    if (!outerBound.contains(currentNode) || currentNode == null) {
      return null;
    }
    currentNode = currentNode.parentNode;
  }
  return currentNode;
}

function chunkToQueryPart(chunk: Data, data: string) {
  return `${"cs"[chunk.Direction]}data:"${escape(data)}"`;
}

function onSelectionChange(this: ThisProxy) {
  const stream = useStreamStore();
  const chunks = stream.stream?.Data;
  if (chunks == null) {
    return;
  }
  const selection = document.getSelection();
  this.selectionData.value = "";
  this.selectionQuery.value = "";
  if (
    selection === null ||
    selection.type !== "Range" ||
    selection.isCollapsed
  ) {
    return;
  }

  const streamData = this.streamData;
  const streamDataNode = streamData?.$el as HTMLElement | null;
  if (streamDataNode === null) {
    return;
  }

  // Assume continuous selection across chunks.
  const { startContainer, startOffset } = selection.getRangeAt(0);
  const { endContainer, endOffset } = selection.getRangeAt(
    selection.rangeCount - 1,
  );
  const startChunkIdxString = getFromDataSet(
    streamDataNode,
    startContainer,
    "chunkIdx",
  );
  const endChunkIdxString = getFromDataSet(
    streamDataNode,
    endContainer,
    "chunkIdx",
  );
  if (startChunkIdxString == null || endChunkIdxString == null) {
    return;
  }
  const startChunkIdx = parseInt(startChunkIdxString);
  const endChunkIdx = parseInt(endChunkIdxString);
  const startOffsetString = getFromDataSet(
    streamDataNode,
    startContainer,
    "offset",
  );
  const endOffsetString = getFromDataSet(
    streamDataNode,
    endContainer,
    "offset",
  );
  const startChunkOffset = parseInt(startOffsetString ?? "0") + startOffset;
  const endChunkOffset = parseInt(endOffsetString ?? "0") + endOffset;

  if (startChunkIdx >= chunks.length) {
    return;
  }

  let queryData = "";
  const queryParts = [];
  for (
    let currentChunkIdx = startChunkIdx;
    currentChunkIdx <= endChunkIdx;
    currentChunkIdx++
  ) {
    const chunk = chunks[currentChunkIdx];
    const start = currentChunkIdx === startChunkIdx ? startChunkOffset : 0;
    const end = currentChunkIdx === endChunkIdx ? endChunkOffset : undefined;
    let data = decodeChunkContent(
      chunk,
      this.presentation.value,
      this.urlDecode.value,
    ).substring(start, end);
    if (this.presentation.value === "utf-8") {
      try {
        const bytes = new TextEncoder().encode(data);
        data = bytes.reduce((acc, byte) => {
          return acc + String.fromCharCode(byte);
        }, "");
      } catch (e) {
        console.error("Failed to encode UTF-8 chunk data:", e);
      }
    }
    queryData += data;
    queryParts.push(chunkToQueryPart(chunk, data));
  }
  this.selectionData.value = queryData;
  this.selectionQuery.value = queryParts.join(" then ");
}
