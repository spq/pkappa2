export interface StreamResult {
    Stream: unknown,
    Tags: string[],
}

export interface StreamsResponse {
    Error?: string,
    Debug?: string[],
    Results?: StreamResult[],
    Offset?: number,
    MoreResults?: boolean,
}
