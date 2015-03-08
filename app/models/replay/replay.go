package replay

import (
	"encoding/base64"
	"encoding/json"
	"github.com/revel/revel"
	"io/ioutil"
	"strconv"
)

var activeGame map[string]string
var activeGameKey string
var activeFirstChunk int

func readNumberChunk(key string) int {
	data, exists := activeGame[key]
	if !exists {
		revel.ERROR.Println("Missing number chunk of key: " + key)
		return -1
	}

	num, err := strconv.Atoi(data)
	if err != nil {
		revel.ERROR.Println("Failed to read number data: " + data)
		return -1
	}

	return num
}

func loadToCache(region, gameId string) bool {
	if activeGameKey == region+":"+gameId {
		return true
	}

	filePath := revel.BasePath + "/replays/" + region + "-" + gameId
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		revel.ERROR.Println("Could not read replay file")
		revel.ERROR.Println(err)
		return false
	}

	result := make(map[string]string)
	err = json.Unmarshal(data, &result)
	if err != nil {
		revel.ERROR.Println("Failed to decode json")
		revel.ERROR.Println(err)
		return false
	}

	activeGameKey = region + ":" + gameId
	activeGame = result

	activeFirstChunk = readNumberChunk("firstChunkNumber")

	if activeFirstChunk < 0 {
		activeFirstChunk = 10
	}

	return true
}

func readData(region, gameId, key string) ([]byte, bool) {
	if !loadToCache(region, gameId) {
		return []byte{}, false
	}

	data, exists := activeGame[key]
	if !exists {
		revel.ERROR.Println("Game ID: " + gameId)
		revel.ERROR.Println("Key: " + key + " does not exist in replay!")
		return []byte{}, false
	}

	result, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		revel.ERROR.Println("Game ID: " + gameId)
		revel.ERROR.Println("Error base64 decoding: " + key)
		return []byte{}, false
	}

	return result, true
}

func GetMetadata(region, gameId string) ([]byte, bool) {
	// Force an update
	activeGameKey = ""
	key := "/getGameMetaData/" + region + "/" + gameId
	return readData(region, gameId, key)
}

func GetLastChunkInfo(region, gameId, last string) ([]byte, bool) {
	if last == "0" {
		return readData(region, gameId, "lastChunkData")
	} else {
		return readData(region, gameId, "firstChunkData")
	}
}

func GetGameDataChunk(region, gameId, frame string) ([]byte, bool) {
	key := "/getGameDataChunk/" + region + "/" + gameId + "/" + frame
	return readData(region, gameId, key)
}

func GetKeyFrame(region, gameId, frame string) ([]byte, bool) {
	key := "/getKeyFrame/" + region + "/" + gameId + "/" + frame
	return readData(region, gameId, key)
}
