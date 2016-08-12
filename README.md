# LoL Replay
[![Go Report Card](https://goreportcard.com/badge/github.com/1lann/lol-replay)](https://goreportcard.com/report/github.com/1lann/lol-replay)
[![GoDoc](https://godoc.org/github.com/1lann/lol-replay?status.svg)](https://godoc.org/github.com/1lann/lol-replay)

LoL Replay is a collection of Go packages to record and play back League of Legends games from the spectator endpoint. It is designed to be fast, reliable, and efficient. The modular design allows anyone to write their own recording service in Go and manipulate the recording files.

The packages are:
- **record**: Retrieves data from the League of Legends servers and stores it into a recording.
- **recording**: Encodes and decodes recordings to a file (or any io.ReadWriteSeeker) using a custom binary format which is able to serve data quickly with a small memory footprint.
- **replay**: Serves recordings over HTTP to be played back using the League of Legends client.
- **server**: Contains the runnable HTTP server which has a web interface, automates recordings, and plays back recordings.

## Documentation
If you would like package documentation, check the [GoDoc](https://godoc.org/github.com/1lann/lol-replay).

## Server Setup
LoL Replay currently assumes your client is running OS X and has [LoL Spectator](https://github.com/1lann/LoL-Spectator) installed to copy and paste replay links into. This should hopefully change to support Windows and OS X without additional programs soon.

1. `go get -u github.com/1lann/lol-replay/server`
2. A binary called `server` will be installed to your `$GOPATH/bin`
3. Download the [sample configuration](/server/config.sample.json).
4. Configure to your liking. Note that platform IDs are taken from [here](https://developer.riotgames.com/docs/spectating-games).
5. Server binary usage: `./server [configuration file location]`. If no configuration file location is specified, it will default to `config.json`.
6. The web host will be running at the bind address specified in the configuration file. Try playing a game, and navigating your browser to it.

If you need help, have an issue or want to ask a question, feel free to contact me by [email](mailto:me@chuie.io) or by making an issue on [GitHub](https://github.com/1lann/LoL-Replay/issues).

## Using Docker
The LoL Replay server is also available as an image on Docker. Refer to the [Docker Hub page](https://hub.docker.com/r/1lann/lol-replay/) for more information.

## Screenshots

![Top of LoL Replay's web interface](http://i.imgur.com/cizHmcs.png)

![Bottom of LoL Replay's web interface](http://i.imgur.com/xa6tQWw.png)

## License
lol-replay is licensed under the MIT license which can be found [here](/LICENSE).
