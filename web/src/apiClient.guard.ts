/*
 * Generated type guards for "apiClient.ts".
 * WARNING: Do not manually change this file.
 */
import { Error, SearchResult, SearchResponse, StreamData, Statistics, PcapsResponse, ConvertersResponse, ProcessStderr, PcapOverIPResponse, TagsResponse, GraphResponse, ClientConfig } from "./apiClient";
import { ConfigEvent } from "./stores/websocket";

export function isError(obj: unknown): obj is Error {
    const typedObj = obj as Error
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        typeof typedObj["Error"] === "string"
    )
}

export function isSearchResult(obj: unknown): obj is SearchResult {
    const typedObj = obj as SearchResult
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        Array.isArray(typedObj["Debug"]) &&
        typedObj["Debug"].every((e: any) =>
            typeof e === "string"
        ) &&
        Array.isArray(typedObj["Results"]) &&
        typedObj["Results"].every((e: any) =>
            (e !== null &&
                typeof e === "object" ||
                typeof e === "function") &&
            (e["Stream"] !== null &&
                typeof e["Stream"] === "object" ||
                typeof e["Stream"] === "function") &&
            typeof e["Stream"]["ID"] === "number" &&
            typeof e["Stream"]["Protocol"] === "string" &&
            (e["Stream"]["Client"] !== null &&
                typeof e["Stream"]["Client"] === "object" ||
                typeof e["Stream"]["Client"] === "function") &&
            typeof e["Stream"]["Client"]["Host"] === "string" &&
            typeof e["Stream"]["Client"]["Port"] === "number" &&
            typeof e["Stream"]["Client"]["Bytes"] === "number" &&
            (e["Stream"]["Server"] !== null &&
                typeof e["Stream"]["Server"] === "object" ||
                typeof e["Stream"]["Server"] === "function") &&
            typeof e["Stream"]["Server"]["Host"] === "string" &&
            typeof e["Stream"]["Server"]["Port"] === "number" &&
            typeof e["Stream"]["Server"]["Bytes"] === "number" &&
            typeof e["Stream"]["FirstPacket"] === "string" &&
            typeof e["Stream"]["LastPacket"] === "string" &&
            typeof e["Stream"]["Index"] === "string" &&
            Array.isArray(e["Tags"]) &&
            e["Tags"].every((e: any) =>
                typeof e === "string"
            )
        ) &&
        typeof typedObj["Offset"] === "number" &&
        typeof typedObj["MoreResults"] === "boolean"
    )
}

export function isSearchResponse(obj: unknown): obj is SearchResponse {
    const typedObj = obj as SearchResponse
    return (
        (isError(typedObj) as boolean ||
            isSearchResult(typedObj) as boolean)
    )
}

export function isStreamData(obj: unknown): obj is StreamData {
    const typedObj = obj as StreamData
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        (typedObj["Stream"] !== null &&
            typeof typedObj["Stream"] === "object" ||
            typeof typedObj["Stream"] === "function") &&
        typeof typedObj["Stream"]["ID"] === "number" &&
        typeof typedObj["Stream"]["Protocol"] === "string" &&
        (typedObj["Stream"]["Client"] !== null &&
            typeof typedObj["Stream"]["Client"] === "object" ||
            typeof typedObj["Stream"]["Client"] === "function") &&
        typeof typedObj["Stream"]["Client"]["Host"] === "string" &&
        typeof typedObj["Stream"]["Client"]["Port"] === "number" &&
        typeof typedObj["Stream"]["Client"]["Bytes"] === "number" &&
        (typedObj["Stream"]["Server"] !== null &&
            typeof typedObj["Stream"]["Server"] === "object" ||
            typeof typedObj["Stream"]["Server"] === "function") &&
        typeof typedObj["Stream"]["Server"]["Host"] === "string" &&
        typeof typedObj["Stream"]["Server"]["Port"] === "number" &&
        typeof typedObj["Stream"]["Server"]["Bytes"] === "number" &&
        typeof typedObj["Stream"]["FirstPacket"] === "string" &&
        typeof typedObj["Stream"]["LastPacket"] === "string" &&
        typeof typedObj["Stream"]["Index"] === "string" &&
        Array.isArray(typedObj["Data"]) &&
        typedObj["Data"].every((e: any) =>
            (e !== null &&
                typeof e === "object" ||
                typeof e === "function") &&
            typeof e["Direction"] === "number" &&
            typeof e["Content"] === "string"
        ) &&
        Array.isArray(typedObj["Tags"]) &&
        typedObj["Tags"].every((e: any) =>
            typeof e === "string"
        ) &&
        Array.isArray(typedObj["Converters"]) &&
        typedObj["Converters"].every((e: any) =>
            typeof e === "string"
        ) &&
        typeof typedObj["ActiveConverter"] === "string"
    )
}

export function isStatistics(obj: unknown): obj is Statistics {
    const typedObj = obj as Statistics
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        typeof typedObj["IndexCount"] === "number" &&
        typeof typedObj["IndexLockCount"] === "number" &&
        typeof typedObj["PcapCount"] === "number" &&
        typeof typedObj["ImportJobCount"] === "number" &&
        typeof typedObj["StreamCount"] === "number" &&
        typeof typedObj["StreamRecordCount"] === "number" &&
        typeof typedObj["PacketCount"] === "number" &&
        typeof typedObj["PacketRecordCount"] === "number" &&
        typeof typedObj["MergeJobRunning"] === "boolean" &&
        typeof typedObj["TaggingJobRunning"] === "boolean" &&
        typeof typedObj["ConverterJobRunning"] === "boolean"
    )
}

export function isClientConfig(obj: unknown): obj is ClientConfig {
    const typedObj = obj as ClientConfig
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") && 
        typeof typedObj["AutoInsertLimitToQuery"] === "boolean"
    )
}

export function isPcapsResponse(obj: unknown): obj is PcapsResponse {
    const typedObj = obj as PcapsResponse
    return (
        Array.isArray(typedObj) &&
        typedObj.every((e: any) =>
            (e !== null &&
                typeof e === "object" ||
                typeof e === "function") &&
            typeof e["Filename"] === "string" &&
            typeof e["Filesize"] === "number" &&
            typeof e["PacketTimestampMin"] === "string" &&
            typeof e["PacketTimestampMax"] === "string" &&
            typeof e["ParseTime"] === "string" &&
            typeof e["PacketCount"] === "number"
        )
    )
}

export function isConvertersResponse(obj: unknown): obj is ConvertersResponse {
    const typedObj = obj as ConvertersResponse
    return (
        Array.isArray(typedObj) &&
        typedObj.every((e: any) =>
            (e !== null &&
                typeof e === "object" ||
                typeof e === "function") &&
            typeof e["Name"] === "string" &&
            typeof e["CachedStreamCount"] === "number" &&
            Array.isArray(e["Processes"]) &&
            e["Processes"].every((e: any) =>
                (e !== null &&
                    typeof e === "object" ||
                    typeof e === "function") &&
                typeof e["Running"] === "boolean" &&
                typeof e["ExitCode"] === "number" &&
                typeof e["Pid"] === "number" &&
                typeof e["Errors"] === "number"
            )
        )
    )
}

export function isProcessStderr(obj: unknown): obj is ProcessStderr {
    const typedObj = obj as ProcessStderr
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        typeof typedObj["Pid"] === "number" &&
        Array.isArray(typedObj["Stderr"]) &&
        typedObj["Stderr"].every((e: any) =>
            typeof e === "string"
        )
    )
}

export function isPcapOverIPResponse(obj: unknown): obj is PcapOverIPResponse {
    const typedObj = obj as PcapOverIPResponse
    return (
        Array.isArray(typedObj) &&
        typedObj.every((e: any) =>
            (e !== null &&
                typeof e === "object" ||
                typeof e === "function") &&
            typeof e["Address"] === "string" &&
            typeof e["LastConnected"] === "number" &&
            typeof e["LastDisconnected"] === "number" &&
            typeof e["ReceivedPackets"] === "number"
        )
    )
}

export function isTagsResponse(obj: unknown): obj is TagsResponse {
    const typedObj = obj as TagsResponse
    return (
        Array.isArray(typedObj) &&
        typedObj.every((e: any) =>
            (e !== null &&
                typeof e === "object" ||
                typeof e === "function") &&
            typeof e["Name"] === "string" &&
            typeof e["Definition"] === "string" &&
            typeof e["Color"] === "string" &&
            typeof e["MatchingCount"] === "number" &&
            typeof e["UncertainCount"] === "number" &&
            typeof e["Referenced"] === "boolean" &&
            Array.isArray(e["Converters"]) &&
            e["Converters"].every((e: any) =>
                typeof e === "string"
            )
        )
    )
}

export function isGraphResponse(obj: unknown): obj is GraphResponse {
    const typedObj = obj as GraphResponse
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        typeof typedObj["Min"] === "string" &&
        typeof typedObj["Max"] === "string" &&
        typeof typedObj["Delta"] === "number" &&
        Array.isArray(typedObj["Aspects"]) &&
        typedObj["Aspects"].every((e: any) =>
            typeof e === "string"
        ) &&
        Array.isArray(typedObj["Data"]) &&
        typedObj["Data"].every((e: any) =>
            (e !== null &&
                typeof e === "object" ||
                typeof e === "function") &&
            Array.isArray(e["Tags"]) &&
            e["Tags"].every((e: any) =>
                typeof e === "string"
            ) &&
            Array.isArray(e["Data"]) &&
            e["Data"].every((e: any) =>
                Array.isArray(e) &&
                e.every((e: any) =>
                    typeof e === "number"
                )
            )
        )
    )
}
