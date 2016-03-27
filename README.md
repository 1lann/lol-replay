# lol-replay

**Notice: This project is still a work-in-progress and its specifications may change at any time. It may also be unreliable and may not work correctly yet.**

lol-replay is a collection of Go packages to record and play back League of Legends games from the spectator endpoint. It is designed to be fast, reliable, and efficient. The modular design allows anyone to write their own recording service in Go and manipulate the recording files.

The packages are:
- **record**: Retrieves data from the League of Legends servers and stores it into a recording.
- **recording**: Encodes and decodes recordings to a file (or any io.ReadWriteSeeker) using a custom binary format which is able to serve data quickly with a small memory footprint.
- **replay**: Serves recordings over HTTP to be played back using the League of Legends client.
- **server**: Contains the runnable HTTP server which has a web interface, automates recordings, and plays back recordings.

## License
lol-replay is licensed under the MIT license which can be found [here](/LICENSE).
