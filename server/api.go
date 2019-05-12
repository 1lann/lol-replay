package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type apiPlayer struct {
	ProfileIconID int    `json:"profile_icon_id"`
	SummonerName  string `json:"summoner_name"`
	SummonerID    string `json:"summoner_id"`
	ChampionName  string `json:"champion_name"`
	ChampionID    int    `json:"champion_id"`
}

type apiRecording struct {
	Region        string      `json:"region"`
	RecordTime    time.Time   `json:"record_time"`
	LastWriteTime time.Time   `json:"last_write_time"`
	IsRecording   bool        `json:"is_recording"`
	ReplayString  string      `json:"replay_string"`
	Players       []apiPlayer `json:"players"`
	Queue         string      `json:"queue"`
}

func writeLastGames(skip int, games int, r *http.Request, w io.Writer) {
	recordingsMutex.RLock()
	n := games
	if len(sortedRecordings) < games {
		n = len(sortedRecordings)
	}

	var recordings []apiRecording

	if skip >= n {
		w.Write([]byte("[]"))
		recordingsMutex.RUnlock()
		return
	}

	func() {
		defer recordingsMutex.RUnlock()

		for i := len(sortedRecordings) - 1; i >= len(sortedRecordings)-n; i-- {
			rec := sortedRecordings[i].rec

			var game gameInfoMetadata
			rec.RetrieveUserMetadata(&game)
			info := rec.RetrieveGameInfo()

			replayCode := "replay " + strings.Split(r.Host, ":")[0] + ":" +
				strconv.Itoa(config.ShowReplayPortAs) + " " + info.EncryptionKey +
				" " + info.GameID + " " + info.Platform

			thisRecording := apiRecording{
				Region:        info.Platform,
				RecordTime:    info.RecordTime,
				LastWriteTime: rec.LastWriteTime(),
				IsRecording:   sortedRecordings[i].recording,
				ReplayString:  replayCode,
				Queue:         getQueue(game.GameQueueConfigID),
			}

			for _, player := range game.Participants {
				championName := "Missing name"
				if champion, found := allChampions[player.ChampionID]; found {
					championName = champion.Name
				}

				thisRecording.Players = append(thisRecording.Players, apiPlayer{
					ProfileIconID: player.ProfileIconID,
					SummonerName:  player.SummonerName,
					SummonerID:    player.SummonerID,
					ChampionName:  championName,
					ChampionID:    player.ChampionID,
				})
			}

			recordings = append(recordings, thisRecording)
		}
	}()

	enc := json.NewEncoder(w)
	err := enc.Encode(recordings)
	if err != nil {
		log.Println("error responding to API request:", err)
	}

	return
}
