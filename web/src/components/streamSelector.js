import ListenerBag from "./listenerBag";

const listenerBag = new ListenerBag();

export function registerSelectionListener(streamInstance) {
  listenerBag.addListener(
    document,
    "selectionchange",
    onSelectionChange.bind(streamInstance)
  );
}

export function destroySelectionListener() {
  listenerBag.clear();
}

function getFromDataSet(outerBound, container, data, fallback = null) {
  const node = getDataSetContainer(outerBound, container, data);
  if (node == null) return fallback;
  return node.dataset[data] ?? fallback;
}

function getDataSetContainer(outerBound, container, data, fallback = null) {
  let currentNode = container;
  while (currentNode?.dataset?.[data] == null) {
    if (!outerBound.contains(currentNode) || currentNode == null) {
      return fallback;
    }
    currentNode = currentNode.parentNode;
  }
  return currentNode;
}

function escape(text) {
  return text
    .split("")
    .map((char) =>
      char.replace(
        /[^ !#$%&',-/0123456789:;<=>ABCDEFGHIJKLMNOPQRSTUVWXYZ^_`abcdefghijklmnopqrstuvwxyz~]/,
        (match) =>
          `\\x{${match
            .charCodeAt(0)
            .toString(16)
            .toUpperCase()
            .padStart("2", "0")}}`
      )
    )
    .join("");
}

function chunkToQueryPart(chunk, data) {
  return `${"cs"[chunk.Direction]}data:"${escape(data)}"`;
}

function onSelectionChange() {
  const selection = document.getSelection();
  if (selection.type !== "Range" || selection.isCollapsed) {
    this.selectionData = "";
    this.selectionQuery = "";
    return;
  }
  const streamDataNode = this.$refs.streamData?.$el ?? this.$refs.streamData;
  if (selection.rangeCount !== 1 || streamDataNode == null) {
    this.selectionData = "";
    this.selectionQuery = "";
    return;
  }
  let { startContainer, startOffset, endContainer, endOffset } =
    selection.getRangeAt(0);
  if (startOffset != 0)
    startContainer = getDataSetContainer(
      streamDataNode,
      startContainer,
      "offset"
    )?.nextSibling;
  if (endOffset != 1)
    endContainer = getDataSetContainer(
      streamDataNode,
      endContainer,
      "offset"
    )?.previousSibling;
  if (
    !streamDataNode.contains(startContainer) ||
    !streamDataNode.contains(endContainer)
  ) {
    this.selectionData = "";
    this.selectionQuery = "";
    return;
  }
  const chunks = this.stream.stream.Data;
  const startChunkIdx = parseInt(
    getFromDataSet(streamDataNode, startContainer, "chunkIdx")
  );
  const startChunkOffset = parseInt(
    getFromDataSet(streamDataNode, startContainer, "offset")
  );
  const endChunkIdx = parseInt(
    getFromDataSet(streamDataNode, endContainer, "chunkIdx")
  );
  const endChunkOffset = parseInt(
    getFromDataSet(streamDataNode, endContainer, "offset")
  );
  if (
    [startChunkIdx, startChunkOffset, endChunkIdx, endChunkOffset].some(
      (i) => i === null
    )
  ) {
    this.selectionData = "";
    this.selectionQuery = "";
    return;
  }

  if (startChunkIdx >= chunks.length) {
    this.selectionData = "";
    this.selectionQuery = "";
    return;
  }

  let queryData = "";
  let queryParts = [];
  for (
    let currentChunkIdx = startChunkIdx;
    currentChunkIdx <= endChunkIdx;
    currentChunkIdx++
  ) {
    const chunk = chunks[currentChunkIdx];
    const start = currentChunkIdx === startChunkIdx ? startChunkOffset : 0;
    const end =
      currentChunkIdx === endChunkIdx ? endChunkOffset + 1 : undefined;
    const data = atob(chunk.Content).substring(start, end);
    queryData += data;
    queryParts.push(chunkToQueryPart(chunk, data));
  }
  this.selectionData = queryData;
  this.selectionQuery = queryParts.join(" then ");
}
