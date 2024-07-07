/*
 * Generated type guards for "websocket.ts".
 * WARNING: Do not manually change this file.
 */
import { Event, TagEvent, ConverterEvent, PcapProcessedEvent } from "./websocket";

export function isEvent(obj: unknown): obj is Event {
    const typedObj = obj as Event
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        typeof typedObj["Type"] === "string"
    )
}

export function isTagEvent(obj: unknown): obj is TagEvent {
    const typedObj = obj as TagEvent
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        (typedObj["Type"] === "tagAdded" ||
            typedObj["Type"] === "tagDeleted" ||
            typedObj["Type"] === "tagUpdated" ||
            typedObj["Type"] === "tagEvaluated") &&
        (typedObj["Tag"] !== null &&
            typeof typedObj["Tag"] === "object" ||
            typeof typedObj["Tag"] === "function") &&
        typeof typedObj["Tag"]["Name"] === "string" &&
        typeof typedObj["Tag"]["Definition"] === "string" &&
        typeof typedObj["Tag"]["Color"] === "string" &&
        typeof typedObj["Tag"]["MatchingCount"] === "number" &&
        typeof typedObj["Tag"]["UncertainCount"] === "number" &&
        typeof typedObj["Tag"]["Referenced"] === "boolean" &&
        Array.isArray(typedObj["Tag"]["Converters"]) &&
        typedObj["Tag"]["Converters"].every((e: any) =>
            typeof e === "string"
        )
    )
}

export function isConverterEvent(obj: unknown): obj is ConverterEvent {
    const typedObj = obj as ConverterEvent
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        (typedObj["Type"] === "converterCompleted" ||
            typedObj["Type"] === "converterDeleted" ||
            typedObj["Type"] === "converterAdded" ||
            typedObj["Type"] === "converterRestarted") &&
        (typedObj["Converter"] !== null &&
            typeof typedObj["Converter"] === "object" ||
            typeof typedObj["Converter"] === "function") &&
        typeof typedObj["Converter"]["Name"] === "string" &&
        typeof typedObj["Converter"]["CachedStreamCount"] === "number" &&
        Array.isArray(typedObj["Converter"]["Processes"]) &&
        typedObj["Converter"]["Processes"].every((e: any) =>
            (e !== null &&
                typeof e === "object" ||
                typeof e === "function") &&
            typeof e["Running"] === "boolean" &&
            typeof e["ExitCode"] === "number" &&
            typeof e["Pid"] === "number" &&
            typeof e["Errors"] === "number"
        )
    )
}

export function isPcapProcessedEvent(obj: unknown): obj is PcapProcessedEvent {
    const typedObj = obj as PcapProcessedEvent
    return (
        (typedObj !== null &&
            typeof typedObj === "object" ||
            typeof typedObj === "function") &&
        typedObj["Type"] === "pcapProcessed" &&
        (typedObj["PcapStats"] !== null &&
            typeof typedObj["PcapStats"] === "object" ||
            typeof typedObj["PcapStats"] === "function") &&
        typeof typedObj["PcapStats"]["PcapCount"] === "number" &&
        typeof typedObj["PcapStats"]["ImportJobCount"] === "number" &&
        typeof typedObj["PcapStats"]["StreamCount"] === "number" &&
        typeof typedObj["PcapStats"]["PacketCount"] === "number"
    )
}
