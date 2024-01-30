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

/**
 * @param {Node} outerBound The outer bound of the search
 * @param {Node} container The current node to search
 * @param {string} data The data attribute to search for
 * @returns {Node|null} The closest parent with the given data attribute or null if none is found
 **/
function getDataSetContainer(outerBound, container, data) {
  let currentNode = container;
  while (currentNode?.dataset?.[data] == null) {
    if (!outerBound.contains(currentNode) || currentNode == null) {
      return null;
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
  this.selectionData = "";
  this.selectionQuery = "";
  if (selection.type !== "Range" || selection.isCollapsed) {
    return;
  }
  const streamDataNode = this.$refs.streamData?.$el ?? this.$refs.streamData;
  // Do not support multi-range selection
  if (selection.rangeCount !== 1 || streamDataNode == null) {
    return;
  }
  let { startContainer, startOffset, endContainer, endOffset } =
    selection.getRangeAt(0);
  endOffset--; // The last character is not selected
  startContainer = getDataSetContainer(
    streamDataNode,
    startContainer,
    "chunkIdx"
  );
  endContainer = getDataSetContainer(streamDataNode, endContainer, "chunkIdx");
  if (
    !streamDataNode.contains(startContainer) ||
    !streamDataNode.contains(endContainer)
  ) {
    return;
  }
  const chunks = this.stream.stream.Data;
  const startChunkIdx = parseInt(
    getFromDataSet(streamDataNode, startContainer, "chunkIdx")
  );
  const endChunkIdx = parseInt(
    getFromDataSet(streamDataNode, endContainer, "chunkIdx")
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
  let queryParts = [];
  for (
    let currentChunkIdx = startChunkIdx;
    currentChunkIdx <= endChunkIdx;
    currentChunkIdx++
  ) {
    const chunk = chunks[currentChunkIdx];
    const start = currentChunkIdx === startChunkIdx ? startOffset : 0;
    const end = currentChunkIdx === endChunkIdx ? endOffset + 1 : undefined;
    const data = atob(chunk.Content).substring(start, end);
    queryData += data;
    queryParts.push(chunkToQueryPart(chunk, data));
  }
  this.selectionData = queryData;
  this.selectionQuery = queryParts.join(" then ");
}
