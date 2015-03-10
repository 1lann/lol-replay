package record

import (
	"encoding/json"
	"github.com/revel/revel"
	"io/ioutil"
	"net/http"
)

type Metadata struct {
	StartupChunk int `json:"endStartupChunkId"`
	LastChunk    int `json:"lastChunkId"`
}

// {"chunkId":27,"availableSince":51192,"nextAvailableChunk":8815,"keyFrameId":11,
// "nextChunkId":26,"endStartupChunkId":4,"startGameChunkId":6,"endGameChunkId":0,"duration":29981}

type ChunkInfo struct {
	NextChunk       int `json:"nextChunkId"`
	CurrentChunk    int `json:"chunkId"`
	NextUpdate      int `json:"nextAvailableChunk"`
	StartGameChunk  int `json:"startGameChunkId"`
	CurrentKeyFrame int `json:"keyFrameId"`
	EndGameChunk    int `json:"endGameChunkId"`
	AvailableSince  int `json:"availableSince"`
	Duration        int `json:"duration"`
	EndStartupChunk int `json:"endStartupChunkId"`
}

func UpdateVersion(url string) {
	resp, ok := requestURL(url + "/observer-mode/rest/consumer/version")
	if ok {
		version = string(resp)
	}
}

func GetVersion() string {
	if version == "" {
		UpdateVersion("http://spectator.oc1.lol.riotgames.com:80")
		return version
	} else {
		return version
	}
}

// Also used in monitor
func requestURL(url string) ([]byte, bool) {
	for i := 0; i < 3; i++ {
		resp, err := http.Get(url)
		if err != nil {
			revel.ERROR.Println("Failed to get: " + url)
			revel.ERROR.Println(err)
			return []byte{}, false
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			revel.WARN.Println("Not 200, status code: " + resp.Status)
			continue
		}

		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			revel.ERROR.Println("Failed to read content from: " + url)
			revel.ERROR.Println(err)
			return []byte{}, false
		}

		return contents, true
	}

	revel.ERROR.Println("Three attempts failed: " + url)
	return []byte{}, false
}

func getMetadata(region, gameId string) Metadata {
	url := platformURLs[region]
	resp, ok := requestURL(url +
		"/observer-mode/rest/consumer/getGameMetaData/" + region +
		"/" + gameId + "/0/token")
	if !ok {
		panic("Failed to get metadata!")
	}

	writeRecording(region, gameId, "/getGameMetaData/"+region+
		"/"+gameId, resp)

	revel.INFO.Println(string(resp))

	result := Metadata{}
	err := json.Unmarshal(resp, &result)
	if err != nil {
		revel.ERROR.Println("Failed to decode json: " + string(resp))
		revel.ERROR.Println(err)
	}
	return result
}

func getChunkFrame(region, gameId, frame string) {
	if frame == "0" {
		return
	}

	recordKey := "/getGameDataChunk/" + region + "/" + gameId + "/" + frame
	if existsRecording(region, gameId, recordKey) {
		return
	}

	url := platformURLs[region]
	resp, ok := requestURL(url +
		"/observer-mode/rest/consumer/getGameDataChunk/" + region + "/" +
		gameId + "/" + frame + "/token")
	if !ok {
		revel.ERROR.Println("Error getting chunk frame: " + frame)
		return
	}

	writeRecording(region, gameId, recordKey, resp)
}

func getKeyFrame(region, gameId, frame string) {
	if frame == "0" {
		return
	}

	recordKey := "/getKeyFrame/" + region + "/" + gameId + "/" + frame
	if existsRecording(region, gameId, recordKey) {
		return
	}

	url := platformURLs[region]
	resp, ok := requestURL(url +
		"/observer-mode/rest/consumer/getKeyFrame/" + region + "/" +
		gameId + "/" + frame + "/token")
	if !ok {
		revel.ERROR.Println("Error getting key frame: " + frame)
		return
	}

	writeRecording(region, gameId, recordKey, resp)
}

// This function does NOT record, it's required for simulating the client
func getLastChunkInfo(region, gameId string) ChunkInfo {
	url := platformURLs[region]
	resp, ok := requestURL(url +
		"/observer-mode/rest/consumer/getLastChunkInfo/" + region + "/" +
		gameId + "/0/token")
	if !ok {
		revel.ERROR.Println("Error getting chunk info")
		return ChunkInfo{}
	}

	result := ChunkInfo{}
	err := json.Unmarshal(resp, &result)
	if err != nil {
		revel.ERROR.Println("Failed to decode json: " + string(resp))
		revel.ERROR.Println(err)
		return ChunkInfo{}
	}

	return result
}
