/*
 * Generated type guards for "websocket.ts".
 * WARNING: Do not manually change this file.
 */
import { Event, TagEvent } from "./websocket";

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
