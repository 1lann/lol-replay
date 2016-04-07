package main

import (
	"encoding/json"
	"github.com/1lann/lol-replay/record"
	"github.com/1lann/lol-replay/recording"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"time"
)

var platformToRegion = map[string]string{
	"NA1":  "na",
	"OC1":  "oce",
	"EUN1": "eune",
	"EUW1": "euw",
	"KR":   "kr",
	"BR1":  "br",
	"LA1":  "lan",
	"LA2":  "las",
	"RU":   "ru",
	"TR1":  "tr",
	"PBE1": "pbe",
}

type player struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
}

type gameInfoMetadata struct {
	BannedChampions []struct {
		ChampionID int `json:"championId"`
		PickTurn   int `json:"pickTurn"`
		TeamID     int `json:"teamId"`
	} `json:"bannedChampions"`
	GameID            int64  `json:"gameId"`
	GameLength        int    `json:"gameLength"`
	GameMode          string `json:"gameMode"`
	GameQueueConfigID int    `json:"gameQueueConfigId"`
	GameStartTime     int64  `json:"gameStartTime"`
	GameType          string `json:"gameType"`
	MapID             int    `json:"mapId"`
	Observers         struct {
		EncryptionKey string `json:"encryptionKey"`
	} `json:"observers"`
	Participants []struct {
		Bot        bool `json:"bot"`
		ChampionID int  `json:"championId"`
		Masteries  []struct {
			MasteryID int `json:"masteryId"`
			Rank      int `json:"rank"`
		} `json:"masteries"`
		ProfileIconID int `json:"profileIconId"`
		Runes         []struct {
			Count  int `json:"count"`
			RuneID int `json:"runeId"`
		} `json:"runes"`
		Spell1Id     int    `json:"spell1Id"`
		Spell2Id     int    `json:"spell2Id"`
		SummonerID   int64  `json:"summonerId"`
		SummonerName string `json:"summonerName"`
		TeamID       int    `json:"teamId"`
	} `json:"participants"`
	PlatformID string `json:"platformId"`
}

func monitorPlayers() {
	waitSeconds := float64(config.RefreshRate) / float64(len(config.Players))
	waitPeriod := time.Millisecond * time.Duration(waitSeconds*1000.0)

	for {
		for _, player := range config.Players {
			time.Sleep(waitPeriod)

			info, ok := player.currentGameInfo(config.RiotAPIKey)
			if !ok {
				continue
			}

			gameId := strconv.FormatInt(info.GameID, 10)
			keyName := info.PlatformID + "_" + gameId
			resume := false

			recordingsMutex.RLock()

			if _, found := recordings[keyName]; found {
				if recordings[keyName].temporary ||
					recordings[keyName].recording ||
					recordings[keyName].rec.IsComplete() {
					recordingsMutex.RUnlock()
					continue
				}

				if !recordings[keyName].rec.IsComplete() {
					resume = true
				}
			}

			recordingsMutex.RUnlock()
			recordingsMutex.Lock()

			if !resume {
				recordings[keyName] = &internalRecording{
					temporary: true,
					recording: false,
				}
			} else {
				recordings[keyName].temporary = true
				recordings[keyName].recording = false
			}

			cleanUp()
			recordingsMutex.Unlock()
			go recordGame(info, resume)
		}
	}
}

// recordingsMutex must be Locked before cleanUp is called.
func cleanUp() {
	for len(recordings) >= config.KeepNumRecordings {
		deleteRecording := sortedRecordings[0]
		deleteRecording.temporary = true
		deleteRecording.file.Close()
		err := os.Remove(deleteRecording.location)
		if err != nil {
			log.Println("failed to delete "+
				deleteRecording.location+":", err)
		}

		sortedRecordings = sortedRecordings[1:]

		for key, rec := range recordings {
			if rec == deleteRecording {
				delete(recordings, key)
				return
			}
		}
	}
}

func recordGame(info gameInfoMetadata, resume bool) {
	gameId := strconv.FormatInt(info.GameID, 10)
	keyName := info.PlatformID + "_" + gameId

	defer func() {
		if e := recover(); e != nil {
			log.Printf("record game panic: %s: %s", e, debug.Stack())

			recordingsMutex.Lock()
			recordings[keyName].recording = false
			recordingsMutex.Unlock()
		}
	}()

	var file *os.File
	var rec *recording.Recording
	var err error
	sortedKey := -1

	if !resume {
		file, err = os.Create(config.RecordingsDirectory + "/" + keyName + ".glr")
		if err != nil {
			log.Println("create recording error:", err)
			return
		}

		rec, err = recording.NewRecording(file)
		if err != nil {
			log.Println("failed to initialize recording:", err)
			return
		}
	} else {
		recordingsMutex.RLock()
		rec = recordings[keyName].rec
		file = recordings[keyName].file

		for i, internalRec := range sortedRecordings {
			if internalRec.rec == rec {
				sortedKey = i
				break
			}
		}
		recordingsMutex.RUnlock()
	}

	recordingsMutex.Lock()
	recordings[keyName] = &internalRecording{
		file:      file,
		location:  config.RecordingsDirectory + "/" + file.Name(),
		rec:       rec,
		temporary: false,
		recording: true,
	}

	if resume && sortedKey >= 0 {
		sortedRecordings[sortedKey] = recordings[keyName]
	} else {
		sortedRecordings = append(sortedRecordings, recordings[keyName])
	}
	recordingsMutex.Unlock()

	if !rec.HasUserMetadata() {
		if err := rec.StoreUserMetadata(&info); err != nil {
			log.Println("recording failed to store game metadata:", err)
			return
		}
	}

	if resume {
		log.Println("resuming recording " + keyName)
	} else {
		log.Println("recording " + keyName)
	}

	err = record.Record(info.PlatformID, gameId,
		info.Observers.EncryptionKey, rec)

	recordingsMutex.Lock()
	recordings[keyName].recording = false
	recordingsMutex.Unlock()

	if err != nil {
		log.Println("error while recording "+keyName+":", err)
		return
	}

	log.Println("recording " + keyName + " complete")
}

func (p player) currentGameInfo(apiKey string) (gameInfoMetadata, bool) {
	url := "https://" + platformToRegion[p.Platform] + ".api.pvp.net/observer-mode/rest" +
		"/consumer/getSpectatorGameInfo/" + p.Platform + "/" + p.ID +
		"?api_key=" + apiKey

	for i := 0; i < 3; i++ {
		resp, err := http.Get(url)
		if err != nil {
			log.Println("URL:", url)
			log.Println("current game error:", err)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return gameInfoMetadata{}, false
		}

		if resp.StatusCode != http.StatusOK {
			log.Println("URL:", url)
			log.Println("current game: not OK:", resp.Status)
			resp.Body.Close()
			continue
		}

		dec := json.NewDecoder(resp.Body)
		var info gameInfoMetadata
		err = dec.Decode(&info)
		resp.Body.Close()
		if err != nil {
			return gameInfoMetadata{}, false
		}

		return info, true
	}

	return gameInfoMetadata{}, false
}
