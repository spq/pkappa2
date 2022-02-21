import ListenerBag from "./listenerBag";

const listenerBag = new ListenerBag();

export function registerSelectionListener(streamInstance) {
    listenerBag.addListener(document, 'selectionchange', onSelectionChange.bind(streamInstance));
}

export function destroySelectionListener() {
    listenerBag.clear();
}

// function base64ToAscii(b64) {
//     const ui8 = Uint8Array.from(
//         new TextEncoder().encode(atob(b64))
//     );

//     return new TextDecoder('ascii').decode(ui8);
// }

function base64ToAscii(b64) {
    return atob(b64);
}

function getFromDataSet(outerBound, container, data, fallback = null) {
    let currentNode = container;
    while (currentNode?.dataset?.[data] == null) {
        if (!outerBound.contains(currentNode) || currentNode == null) {
            return fallback;
        }
        currentNode = currentNode.parentNode;
    }

    return currentNode.dataset[data] ?? fallback;
}

function escape(text) {
    return text
        .split("")
        .map(char => String.fromCharCode(char.charCodeAt(0))
            .replace(
                /\W/,
                (match) => '\\x{' + match.charCodeAt(0).toString(16).toUpperCase().padStart('2', '0') + '}'
            )
        )
        .join('');
}

function chunkToQueryPart(chunk, start, length = undefined) {
    return (chunk.Direction === 0 ? 'cdata' : 'sdata') + ':"' + escape(base64ToAscii(chunk.Content).substring(start, length)) + '"';
}

function onSelectionChange() {
    const selection = document.getSelection();
    const streamDataNode = this.$refs.streamData?.$el ?? this.$refs.streamData;
    if (selection.rangeCount !== 1 || streamDataNode == null) {
        return;
    }
    const { startContainer, endContainer } = selection.getRangeAt(0);
    if (!streamDataNode.contains(startContainer) || !streamDataNode.contains(endContainer)) {
        return;
    }
    const chunks = this.stream.stream.Data;
    const startChunkIdx = parseInt(getFromDataSet(streamDataNode, startContainer, 'chunkIdx', 0));
    const startOffset = parseInt(getFromDataSet(streamDataNode, startContainer, 'offset', 0));
    const endChunkIdx = parseInt(getFromDataSet(streamDataNode, endContainer, 'chunkIdx', chunks.length - 1));
    const endOffset = parseInt(getFromDataSet(streamDataNode, endContainer, 'offset', chunks[chunks.length - 1].Content.length));

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
    /** @TODO: Change back to `then` behaviour when bug is fixed */
    //this.selectionQuery = queryParts.join(' then ');
    this.selectionQuery = queryParts.join(' ');
}
