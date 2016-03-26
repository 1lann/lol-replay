package main

import (
	"encoding/json"
	"github.com/1lann/lol-replay/record"
	"github.com/1lann/lol-replay/recording"
	"log"
	"net/http"
	"os"
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

type gameInfo struct {
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

func monitorPlayers(config configuration) {
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
			if _, found := recordings[info.PlatformID+"_"+gameId]; found {
				continue
			}

			recordings[info.PlatformID+"_"+gameId] = &internalRecording{
				temporary: true,
				recording: false,
			}

			// TODO: Clean up and close file handler

			go recordGame(config, info)
		}
	}
}

func recordGame(config configuration, info gameInfo) {
	gameId := strconv.FormatInt(info.GameID, 10)
	keyName := info.PlatformID + "_" + gameId

	file, err := os.Create(config.RecordingsDirectory + "/" + keyName + ".glr")
	if err != nil {
		log.Println("create recording error:", err)
		return
	}

	rec, err := recording.NewRecording(file)
	if err != nil {
		log.Println("failed to initialize recording:", err)
		return
	}

	recordings[keyName] = &internalRecording{
		file:      file,
		location:  config.RecordingsDirectory + "/" + file.Name(),
		rec:       rec,
		temporary: false,
		recording: true,
	}

	log.Println("recording " + keyName)

	err = record.Record(info.PlatformID, gameId,
		info.Observers.EncryptionKey, info, rec)

	if err != nil {
		log.Println("error while recording "+keyName+":", err)
		return
	}
}

func (p player) currentGameInfo(apiKey string) (gameInfo, bool) {
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
			return gameInfo{}, false
		}

		if resp.StatusCode != http.StatusOK {
			log.Println("URL:", url)
			log.Println("current game: not OK:", resp.Status)
			resp.Body.Close()
			continue
		}

		dec := json.NewDecoder(resp.Body)
		var info gameInfo
		err = dec.Decode(&info)
		resp.Body.Close()
		if err != nil {
			return gameInfo{}, false
		}

		return info, true
	}

	return gameInfo{}, false
}
