import ListenerBag from "./listenerBag";
import { useStreamStore } from "@/stores/stream";
import Vue, { Ref } from "vue";
import { Data } from "@/apiClient";

const listenerBag = new ListenerBag();

type ThisProxy = {
  proxy: Vue;
  selectionData: Ref<string>;
  selectionQuery: Ref<string>;
};

export function registerSelectionListener(streamInstance: ThisProxy) {
  listenerBag.addListener(
    document,
    "selectionchange",
    onSelectionChange.bind(streamInstance)
  );
}

export function destroySelectionListener() {
  listenerBag.clear();
}

function getFromDataSet(
  outerBound: Node,
  container: Node,
  data: string,
  fallback = null
) {
  const node = getDataSetContainer(outerBound, container, data);
  if (node == null) return fallback;
  return node.dataset[data] ?? fallback;
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

function escape(text: string) {
  return text
    .split("")
    .map((char) =>
      char.replace(
        /[^ !#%&',/0123456789:;<=>ABCDEFGHIJKLMNOPQRSTUVWXYZ_`abcdefghijklmnopqrstuvwxyz~-]/,
        (match) =>
          `\\x{${match
            .charCodeAt(0)
            .toString(16)
            .toUpperCase()
            .padStart(2, "0")}}`
      )
    )
    .join("");
}

function chunkToQueryPart(chunk: Data, data: string) {
  return `${"cs"[chunk.Direction]}data:"${escape(data)}"`;
}

function onSelectionChange(this: ThisProxy) {
  const stream = useStreamStore();
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

  const streamData = this.proxy.$refs.streamData;
  if (
    streamData instanceof Element ||
    Array.isArray(streamData) ||
    streamData == null
  ) {
    return;
  }
  const streamDataNode = streamData.$el;
  // Do not support multi-range selection
  if (selection.rangeCount !== 1 || streamDataNode == null) {
    return;
  }
  const { startContainer, startOffset, endContainer, endOffset } =
    selection.getRangeAt(0);
  const datasetStartContainer = getDataSetContainer(
    streamDataNode,
    startContainer,
    "chunkIdx"
  );
  const datasetEndContainer = getDataSetContainer(
    streamDataNode,
    endContainer,
    "chunkIdx"
  );
  if (
    datasetStartContainer == null ||
    datasetEndContainer == null ||
    !streamDataNode.contains(datasetStartContainer) ||
    !streamDataNode.contains(datasetEndContainer)
  ) {
    return;
  }
  const chunks = stream.stream?.Data;
  if (chunks == null) {
    return;
  }
  const startChunkIdx = parseInt(
    getFromDataSet(streamDataNode, datasetStartContainer, "chunkIdx") ?? "0"
  );
  const endChunkIdx = parseInt(
    getFromDataSet(streamDataNode, datasetEndContainer, "chunkIdx") ?? "0"
  );
  if (
    [startChunkIdx, startOffset, endChunkIdx, endOffset].some((i) => i === null)
  ) {
    return;
  }

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
    const start = currentChunkIdx === startChunkIdx ? startOffset : 0;
    const end = currentChunkIdx === endChunkIdx ? endOffset : undefined;
    const data = atob(chunk.Content).substring(start, end);
    queryData += data;
    queryParts.push(chunkToQueryPart(chunk, data));
  }
  this.selectionData.value = queryData;
  this.selectionQuery.value = queryParts.join(" then ");
}
