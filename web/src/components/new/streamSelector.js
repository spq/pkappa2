import ListenerBag from "./listenerBag";

const listenerBag = new ListenerBag();

export function registerSelectionListener(streamInstance) {
    listenerBag.addListener(document, 'selectionchange', onSelectionChange.bind(streamInstance));
}

export function destroySelectionListener() {
    listenerBag.clear();
}

function base64ToAscii(b64) {
    const ui8 = Uint8Array.from(
        atob(b64)
            .split("")
            .map((char) => char.charCodeAt(0))
    );

    return new TextDecoder().decode(ui8);
}

function getFromDataSet(outerBound, container, data) {
    let currentNode = container;
    while (currentNode?.dataset?.[data] == null) {
        if (!outerBound.contains(currentNode) || currentNode == null) {
            return false;
        }
        currentNode = currentNode.parentNode;
    }

    return currentNode.dataset[data];
}

function escape(text) {
    return (text
        // eslint-disable-next-line no-control-regex
        .replaceAll(/[\x00-\x1F\x80-\xFF"{}@[\]]/g, (match) => '\\x' + match.charCodeAt(0).toString(16).padStart('2', '0'))
        .replaceAll('"', '""')
    );
}

function chunkToQueryPart(chunk, start, length = undefined) {
    return (chunk.Direction === 0 ? 'cdata' : 'sdata') + ':"' + escape(base64ToAscii(chunk.Content).substring(start, length)) + '"';
}

function onSelectionChange() {
    const selection = document.getSelection();
    const streamDataNode = this.$refs.streamData?.$el ?? this.$refs.streamData;
    const { startContainer, endContainer } = selection.getRangeAt(0);
    if (selection.rangeCount !== 1 || streamDataNode == null || !streamDataNode.contains(startContainer) || !streamDataNode.contains(endContainer)) {
        return;
    }
    const startChunkIdx = parseInt(getFromDataSet(streamDataNode, startContainer, 'chunkIdx'));
    const startOffset = parseInt(getFromDataSet(streamDataNode, startContainer, 'offset'));
    const endChunkIdx = parseInt(getFromDataSet(streamDataNode, endContainer, 'chunkIdx'));
    const endOffset = parseInt(getFromDataSet(streamDataNode, endContainer, 'offset'));
    const chunks = this.stream.stream.Data;

    if (startChunkIdx >= chunks.length) {
        return;
    }

    let queryParts = [];
    for (let currentChunkIdx = startChunkIdx; currentChunkIdx <= endChunkIdx; currentChunkIdx++) {
        const from = currentChunkIdx === startChunkIdx ? startOffset : 0;
        queryParts.push(chunkToQueryPart(
            chunks[currentChunkIdx], 
            from,
            currentChunkIdx === endChunkIdx ? endOffset : undefined
        ));
    }
    this.selectionQuery = queryParts.join(' then ');
}
