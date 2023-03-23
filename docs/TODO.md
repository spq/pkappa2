# pkappa2 TODO

## server:
- [x] support http
- [x] support http/1 websocket, including compression
- [x] support http/2
- [x] support http/2 websocket
- [ ] support quic
- [ ] support ocsp
- [ ] support is:started|finished
- [ ] support pcap groups, they have their own indexes & snapshots and may only be combined with packets in the same group
- [ ] fix ip4 defragmentation (snapshottable, list of packets that are source for a reassembled pkg)
- [ ] support ip6 defragmenting
- [ ] support sctp
- [ ] support relative times in tags
- [ ] add tests
- [ ] make query language simpler (less @'s)
- [ ] improve import speed by ignoring timedout packages instead of having to flush them before processing a new package
- [ ] cache matching + uncertain streams for tags

## web:
- [ ] history reverse search with strg+r
- [ ] improve/document graph ui
- [ ] diffing of two streams
- [ ] render http response in iframe with correct content type
- [ ] add button to download raw data of a stream
- [ ] autocomplete keywords while typing the query "nearley unparse"
- [ ] show matching generated marks in stream view
- [x] let large tag queries and names overflow instead of widening the page layout
- [ ] add open in CyberChef button to stream chunks
- [ ] highlight data matches in stream view

## both:
- [ ] add search history overlay for recent searches
- [ ] support showing alternatives for groups
- [ ] support showing sub query results
- [x] add download button for generated python script that replays the stream (https://github.com/secgroup/flower/blob/master/services/flow2pwn.py https://github.com/secgroup/flower/blob/master/services/data2req.py)
- [ ] optional search result snippets
- [ ] support filters for search and display, see below for how
- [ ] calculate levenshtein distance to all previous streams and save the stream id with least difference and the difference
- [ ] add documentation

## filters / converters
the converter feature will be implemented like this:
- [x] converters are executed serverside
  - [x] each executable file in converters/ is considered a converter
    - an example is b64decode.py which decodes every received chunk using b64
  - [x] the protocol uses stdin/stdout, one json object per line or an empty line
    - pkappa sends the following lines to the filter:
      - first: general stream information json
      - one line per data chunk containing a json with the dir(ection) and data(encoded in base64)
      - one empty line terminating the chunks
    - the filter responds with the following lines:
      - one line per output data chunk formatted identical to the ones coming from pkappa
      - one empty line terminating the chunks
      - one general stream information json
- [ ] include matching tags/services/marks in general stream information json?
- [x] tags, marks and services can be triggers for a collection of filters if they have a low complexity
  - [ ] they must not match on filtered-data for now, also indirectly via other tags/marks/services
    - currently they cannot match any data filter. `data.none:` should be allowed.
- [x] whenever pkappa becomes aware of a stream matching a tag/mark/service that triggers a filter but the output of that filter for this stream is not yet cached, it will queue up a filtering processing
  - [ ] all matches are queued up whenever a tag update job finishes. this could be optimized to only queue new / updated matches
- [ ] whenever pkappa becomes aware of a stream no longer matching any tag/mark/service that triggers a filter but there exists a cache for the output of the given filter for the stream, that cached info is invalidated
- [ ] rerun the converter if a stream is updated through new pcaps
- [x] the stream request api will get a parameter for selecting the filter to apply, it will support auto, none, filter:<name>
  - [x] the mode auto is the default and will return the original stream data or the single cached filtered stream (if there is exactly one)
- [x] there will be one cache file per active filter with this format:
  - [x] [u64 stream id] [u8 varint chunk sizes] [client data] [server data]
  - [ ] when the stream id is ~0, this is an unused slot
  - [ ] only save the output if it differs from the plaintext stream data to save space
- [x] the search will be modified this way:
  - [x] [cs]data filters will search in all currently available filtered outputs as well as the unmodified stream content
  - [x] there will be modifiers for these [cs]data filters that allow to specify which of the filtered outputs are searched, or to specify exactly one output that is used
    - The modifier looks like `[cs]data.convertername:content`
    - `none` is a reserved converter name and selects the plain unprocessed stream data
  - [ ] [cs]bytes filters will support specifying the converter modifier too
- [x] when a filter was evaluated tags and services might be re-evaluated when they contain [cs]data filters, thats why those tags/services may not be used as triggers
- [ ] keep stderr and exit code in all cases. keep stderr if stderr not empty, but the process exited as expected?
- [x] show stderr of filters in UI
  - stderr can be fetched from `/api/converters/stderr/[name]`
- [ ] use states in filter json protocol and display which state we're currently in in UI for debugging filter scripts
- [x] name filters transformations? converters? -> `converters` it is
- [x] allow to run any converter for any stream even if not attached to a stream in the stream view
  - this could be used to implement the "stream to pwntools or python requests" generators
  - should indicate if the converter is also attached to one of the tags matching the stream
- [ ] allow converters to add (generated) tags to a stream
- [ ] option to mark converter output "informative" and render it differently than client/server traffic
  - e.g. to render the pwntools script generator output in an easy to copy way without the "client sent" coloring
- [ ] split chunk into sub-chunks with different content-types to e.g. render images inline
