# Pkappa2 Filter Specification

Any executable file in the `filters` directory (`-filter_dir` argument) can be attached to a tag. The filename will be used as an identifier to connect the tags with a selected filter. Filters can be used to postprocess stream data and add the results to streams as an alternative view on the data. The results can be searched like the plain stream content.

The filter program is started once and kept running in the background by pkappa2. The program is expected to accept JSON messages from `stdin` and answers on `stdout` in a loop using the following protocol:

## Protocol
The protocol is text-based exchanging one JSON object per line over `stdin` and `stdout`.
### 1. Pkappa2 -> Filter: Stream metadata
```json
{
    "ClientHost": "10.13.0.1",
    "ClientPort": 45050,
    "ServerHost": "10.1.7.1",
    "ServerPort": 5005,
    "Protocol": "TCP",
} 
```

Currently only `"TCP"` and `"UDP"` protocols are supported.

### 2. Pkappa2 -> Filter: Stream chunks
```json
{
    "Direction": "client-to-server",
    "Content": "R0VUIC8gSFRUUC8xLjENCkhvc3Q6IDEwLjEuNy4xOjUwMDUNCkNvbm5lY3Rpb246IGtlZXAtYWxpdmUNCg0KCg=="
}
{
    "Direction": "server-to-client",
    "Content": "SFRUUC8xLjEgMjAwIE9LDQpDb250ZW50LUxlbmd0aDogMA0KDQoK"
}

```

The stream is split up into chunks sent by the server or the client. Every chunk is sent in individual lines while an empty line signales the end of the list. The content is Base64 encoded.

### 3. Filter -> Pkappa2: Modified stream chunks
```json
{
    "Direction": "client-to-server",
    "Content": "R0VUIC8gSFRUUC8xLjENCkhvc3Q6IDEwLjEuNy4xOjUwMDUNCkNvbm5lY3Rpb246IGtlZXAtYWxpdmUNCg0KCg=="
}
{
    "Direction": "server-to-client",
    "Content": "SFRUUC8xLjEgMjAwIE9LDQpDb250ZW50LUxlbmd0aDogMA0KDQoK"
}

```

The modified chunks are sent in the same format as previously received.

### 4. Filter -> Pkappa2: Additional stream metadata
```json
{}
```

There are no elements supported yet.
