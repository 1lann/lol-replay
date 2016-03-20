# lol-replay

**Notice: This project is still a work-in-progress and its specifications may change at any time. It may also be unreliable and may not work correctly yet.**

A collection of Go packages to record and play back League of Legends games from the spectator endpoint.

The three packages are:
- **record**: Retrieves data from the League of Legends servers.
- **recording**: Encodes and decodes recordings to a file (or any io.ReadWriteSeeker) using a custom binary format which is able to serve data quickly with a small memory footprint.
- **replay**: Serves recordings over HTTP to be played back using the League of Legends client.
