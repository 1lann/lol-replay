package monitor

import (
	"encoding/json"
	"github.com/revel/revel"
	"io/ioutil"
	"net/http"
	"replay/app/models/record"
	"strconv"
	"time"
)

var apiKey string

// onelann, Slowgunmaker, xChryssalidx
var playersTrackedOCE []string = []string{
	"3190144",
	"3790244",
	"3330376",
}

// 1lann, Slowgunmaker, KingCreeper457, calvin1023
var playersTrackedNA []string = []string{
	"24431578",
	"65399212",
	"43769335",
	"59945261",
}

var recordedGames []string

type Observer struct {
	EncryptionKey string `json:"encryptionKey"`
}

type CurrentGame struct {
	Id        float64  `json:"gameId"`
	Observers Observer `json:"observers"`
	Platform  string   `json:"platformId"`
}

// Also used in gettters
func requestURL(url string) ([]byte, bool) {
	resp, err := http.Get(url)
	if err != nil {
		revel.ERROR.Println("Failed to get: " + url)
		revel.ERROR.Println(err)
		return []byte{}, false
	}

	if resp.StatusCode == 404 {
		return []byte{}, false
	}

	if resp.StatusCode != 200 {
		revel.WARN.Println("Not 200 or 404, status code: " + resp.Status)
		return []byte{}, false
	}

	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		revel.ERROR.Println("Failed to read content from: " + url)
		revel.ERROR.Println(err)
		return []byte{}, false
	}

	return contents, true
}

func parseMatch(data []byte) CurrentGame {
	result := CurrentGame{}

	err := json.Unmarshal(data, &result)
	if err != nil {
		revel.ERROR.Println("Failed to parse current game")
		revel.ERROR.Println(string(data))
		revel.ERROR.Println(err)
		return CurrentGame{}
	}

	return result
}

func checkPlayer(platform, player string) {
	resp, ok := requestURL("https://na.api.pvp.net/observer-mode/rest/consumer/getSpectatorGameInfo/" + platform + "/" + player + "?api_key=" + apiKey)
	if !ok {
		return
	}
	game := parseMatch(resp)

	if game.Id > 0 {
		gameId := strconv.FormatFloat(game.Id, 'f', 0, 64)
		record.Record(platform, gameId, game.Observers.EncryptionKey)
	}
}

func runMonitor() {
	for {
		for _, player := range playersTrackedNA {
			time.Sleep(time.Duration(5) * time.Second)
			checkPlayer("NA1", player)
		}
		for _, player := range playersTrackedOCE {
			time.Sleep(time.Duration(5) * time.Second)
			checkPlayer("OC1", player)
		}
	}
}

func startMonitor() {
	testKey, found := revel.Config.String("riotapikey")
	if !found {
		revel.ERROR.Println("Missing API key!")
		panic("Missing API key!")
	}
	apiKey = testKey

	if !revel.DevMode {
		revel.INFO.Println("Monitor started")
		go runMonitor()
	} else {
		revel.INFO.Println("Dev mode, monitor not starting")
	}

}

func init() {
	revel.OnAppStart(startMonitor)
}
