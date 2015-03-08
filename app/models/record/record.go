package record

// /CdsXSnhGV0TQ9B3VZ9IneNK4vk+K9k+

import (
	"encoding/base64"
	"encoding/json"
	"github.com/revel/revel"
	"io/ioutil"
	"replay/app/models/history"
	"strconv"
	"time"
)

var platformURLs map[string]string = map[string]string{
	"NA1":  "http://spectator.na.lol.riotgames.com:80",
	"OC1":  "http://spectator.oc1.lol.riotgames.com:80",
	"EUN1": "http://spectator.eu.lol.riotgames.com:8088",
	"EUW1": "http://spectator.euw1.lol.riotgames.com:80",
}

var version string // Version functions in getters.go
var recording map[string]map[string]string = make(map[string]map[string]string)

func writeRecording(region, gameId, key string, value []byte) {
	currentRecording := recording[region+":"+gameId]
	currentRecording[key] = base64.URLEncoding.EncodeToString(value)
	recording[region+":"+gameId] = currentRecording
}

func writeString(region, gameId, key string, value string) {
	currentRecording := recording[region+":"+gameId]
	currentRecording[key] = value
	recording[region+":"+gameId] = currentRecording
}

func existsRecording(region, gameId, key string) bool {
	if _, exists := recording[region+":"+gameId][key]; exists {
		return true
	}
	return false
}

func writeLastChunkInfo(region, gameId string,
	firstChunk, firstKeyFrame int, chunk ChunkInfo) {

	writeChunk := ChunkInfo{
		NextChunk:       firstChunk,
		CurrentChunk:    firstChunk,
		NextUpdate:      3000,
		StartGameChunk:  chunk.StartGameChunk,
		CurrentKeyFrame: firstKeyFrame,
		EndGameChunk:    chunk.CurrentChunk,
		AvailableSince:  0,
		Duration:        3000,
		EndStartupChunk: chunk.EndStartupChunk,
	}

	result, err := json.Marshal(writeChunk)
	if err != nil {
		panic("Error while encoding first chunk data json?!??!??")
	}

	writeRecording(region, gameId, "firstChunkData", result)

	writeChunk.NextChunk = chunk.CurrentChunk
	writeChunk.CurrentChunk = chunk.CurrentChunk
	writeChunk.CurrentKeyFrame = chunk.CurrentKeyFrame

	result, err = json.Marshal(writeChunk)
	if err != nil {
		panic("Error while encoding last chunk data json?!??!??")
	}

	writeRecording(region, gameId, "lastChunkData", result)
	writeString(region, gameId, "firstChunkNumber", strconv.Itoa(firstChunk))
}

func saveRecording(region, gameId string) {
	savePath := revel.BasePath + "/replays/" + region + "-" + gameId

	result, err := json.Marshal(recording[region+":"+gameId])
	if err != nil {
		panic("Error while encoding recording json?!?!?!?")
	}

	err = ioutil.WriteFile(savePath, result, 0644)
	if err != nil {
		revel.ERROR.Println("Error saving recording!")
	}
}

func recordMetadata(region, gameId string) {
	metadata := getMetadata(region, gameId)

	for {
		chunk := getLastChunkInfo(region, gameId)
		if chunk.CurrentChunk > metadata.StartupChunk {
			break
		}
		time.Sleep(time.Duration(chunk.NextUpdate)*time.Millisecond +
			time.Second)
	}

	metadata = getMetadata(region, gameId)

	// Get the startup frames
	for i := 1; i <= metadata.StartupChunk+1; i++ {
		// revel.INFO.Println("Getting startup chunk:", i)
		for {
			chunk := getLastChunkInfo(region, gameId)
			if i > chunk.CurrentChunk {
				time.Sleep(time.Duration(chunk.NextUpdate)*time.Millisecond +
					time.Second)
				continue
			}
			getChunkFrame(region, gameId, strconv.Itoa(i))
			break
		}
	}
}

func recordFrames(region, gameId string) {
	firstChunk := 0
	firstKeyFrame := 0
	lastChunk := 0
	lastKeyFrame := 0

	for {
		chunk := getLastChunkInfo(region, gameId)

		if firstChunk == 0 {
			if chunk.CurrentChunk > chunk.StartGameChunk {
				firstChunk = chunk.CurrentChunk
			} else {
				firstChunk = chunk.StartGameChunk
			}

			if chunk.CurrentKeyFrame > 0 {
				firstKeyFrame = chunk.CurrentKeyFrame
			} else {
				firstKeyFrame = 1
			}

			lastChunk = chunk.CurrentChunk
			lastKeyFrame = chunk.CurrentKeyFrame

			getChunkFrame(region, gameId, chunk.CurrentChunk)
			getKeyFrame(region, gameId, chunk.CurrentKeyFrame)
		}

		if chunk.CurrentChunk > lastChunk {
			for i := lastChunk + 1; i <= chunk.CurrentChunk; i++ {
				getChunkFrame(region, gameId, strconv.Itoa(i))
			}
		}

		if chunk.NextChunk < chunk.CurrentChunk && chunk.NextChunk > 0 {
			getChunkFrame(region, gameId, strconv.Itoa(chunk.NextChunk))
		}

		if chunk.CurrentKeyFrame > lastKeyFrame {
			for i := lastKeyFrame + 1; i <= chunk.CurrentKeyFrame; i++ {
				getKeyFrame(region, gameId, strconv.Itoa(chunk.CurrentKeyFrame))
			}
		}

		writeLastChunkInfo(region, gameId, firstChunk, firstKeyFrame, chunk)
		saveRecording(region, gameId)

		lastChunk = chunk.CurrentChunk
		lastKeyFrame = chunk.CurrentKeyFrame

		if chunk.EndGameChunk == chunk.CurrentChunk {
			return
		}

		time.Sleep(time.Duration(chunk.NextUpdate)*time.Millisecond +
			time.Second)
	}
}

func asyncRecord(region, gameId, encryptionKey string) {
	defer func() {
		if r := recover(); r != nil {
			revel.ERROR.Println("Error while recording game ID: " + gameId)
			revel.ERROR.Println(r)
			delete(recording, region+":"+gameId)
		}
	}()

	writeRecording(region, gameId, "encryptionKey", []byte(encryptionKey))

	url := platformURLs[region]
	UpdateVersion(url)

	revel.INFO.Println("Now recording: " + region + ":" + gameId)
	revel.INFO.Println(gameId + "'s Encryption Key: " + encryptionKey)

	recordMetadata(region, gameId)
	recordFrames(region, gameId)

	revel.INFO.Println("Recording complete for: " + region + ":" + gameId)
	delete(recording, region+":"+gameId)
}

func Record(region, gameId, encryptionKey string) bool {
	if _, ok := recording[region+":"+gameId]; ok {
		return false
	} else {
		recording[region+":"+gameId] = make(map[string]string)
	}

	history.StoreGame(region, gameId, encryptionKey)

	go asyncRecord(region, gameId, encryptionKey)
	return true
}
