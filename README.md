# Pkappa2

Pkappa2 is a packet stream analysis tool intended for Attack & Defense CTF Competitions.
It receives pcap files via a http upload, usually send by a tcpdump-complete script.
The received pcaps are processed and using the webinterface, users can run queries over the streams.
Streams matching the query are displayed and their content can be viewed in multiple formats.

The tool is under development and might not work!
See [docs/TODO.md](docs/TODO.md) for missing features.

Add pcaps using a POST to `/upload/filename.pcap`:
```
curl --data-binary @some-file.pcap http://localhost:8080/upload/some-file.pcap
```

## Running

- install required dependencies
    - libpcap (e.g. `apt install libpcap-dev`)
- run `yarn install && yarn build` in `/web`
- run `go run cmd/pkappa2/main.go` in `/`
- visit `localhost:8080` in your web browser

You likely want to add some arguments to the `go run` command, check `-help`

### Docker
- copy `.env.example` to `.env` and change the configuration
- run `docker compose up -d`
- visit `localhost:8080` in your web browser

## UI Development

- make sure you can run Pkappa2
- run `yarn serve` in `/web`
- run `go run cmd/pkappa2/main.go -address :8081` in `/`
- visit `localhost:8080` in your web browser
