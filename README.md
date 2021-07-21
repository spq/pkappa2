# Pkappa2

Pkappa2 is a packet stream analysis tool intended for Attack & Defense CTF Competitions.
It receives pcap files via a http upload, usually send by a tcpdump-complete script.
The received pcaps are processed and using the webinterface, users can run queries over the streams.
Streams matching the query are displayed and their content can be viewed in multiple formats.

The tool is under development and might not work!
See docs/TODO.txt for missing features.

## Running

- install required dependencies (TODO: which?)
- run `yarn build` in `/web`
- run `go run cmd/pkappa2/main.go` in `/`
- visit `localhost:8080` in your web browser

You likely want to add some arguments to the `go run` command, check `-help`

## UI Development

- make sure you can run Pkappa2
- run `yarn serve` in `/web`
- run `go run cmd/pkappa2/main.go -address :8081` in `/`
- visit `localhost:8080` in your web browser