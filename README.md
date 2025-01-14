# Pkappa2

[![Go Coverage](https://github.com/spq/pkappa2/wiki/coverage.svg)](https://raw.githack.com/wiki/spq/pkappa2/coverage.html)

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

- requires [go](https://go.dev/dl/) 1.22+
- install required dependencies
    - libpcap (e.g. `apt install libpcap-dev`)
- run `yarn install && yarn build` in `/web`
- run `go run cmd/pkappa2/main.go` in `/`
- optionally, install stock converter python dependencies: `pip install -r converters/pkappa2lib/requirements.txt`
- visit `localhost:8080` in your web browser

You likely want to add some arguments to the `go run` command, check `-help`

### Docker
- copy `.env.example` to `.env` and change the configuration
- run `docker compose up -d`
- visit `localhost:8080` in your web browser

## UI Development

- make sure you can run Pkappa2
- run `yarn dev` in `/web`
- run `go run cmd/pkappa2/main.go -address :8081` in `/`
- visit `localhost:8080` in your web browser

You can import multiple .pcap files in the current folder using:
`for f in *.pcap; do curl --data-binary "@$f" "http://localhost:8081/upload/$f"; done`

## Generating type guards

In order to generate all the typeguards, go to `web/` and call
```
npx ts-auto-guard
```

When getting api-responses about types mismatching, you can debug the typeguards via
```
npx ts-auto-guard --debug
```
