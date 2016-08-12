FROM golang:1.7-alpine
RUN apk add --no-cache git
RUN go get -u github.com/1lann/lol-replay/server
RUN mkdir /lol-replay
RUN echo $'{\n\
        "players": [],\n\
        "recordings_directory": "/lol-replay/recordings",\n\
        "bind_address": "0.0.0.0:9001",\n\
        "riot_api_key": "your Riot API key here",\n\
        "refresh_rate_seconds": 90,\n\
        "keep_num_recordings": 100,\n\
        "show_per_page": 20,\n\
        "show_replay_port_as": 9001\n\
}' > /lol-replay/config.json
VOLUME /lol-replay
CMD /go/bin/server /lol-replay/config.json
EXPOSE 9001
