package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type staticVersions struct {
	Dd string `json:"dd"`
}

type championInfo struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Title string `json:"title"`
	Image struct {
		Full string `json:"full"`
	} `json:"image"`
}

type championData struct {
	Type    string                  `json:"type"`
	Format  string                  `json:"format"`
	Version string                  `json:"version"`
	Data    map[string]championInfo `json:"data"`
}

var currentVersion string
var allChampions = make(map[int]championInfo)
var championsMutex = new(sync.RWMutex)
var staticDataAvailable bool
var staticDataMutex = new(sync.Mutex)

func maintainStaticData() {
	for {
		if updateVersion() {
			updateChampions()
		}
		time.Sleep(time.Hour * 6)
	}
}

func updateVersion() bool {
	resp, err := http.Get("https://ddragon.leagueoflegends.com/realms/oce.json")
	if err != nil {
		log.Println("riot static: failed to get version:", err)
		return false
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("riot static: version not OK:", resp.Status)
		return false
	}

	dec := json.NewDecoder(resp.Body)
	var version staticVersions
	if err := dec.Decode(&version); err != nil {
		log.Println("riot static: version decode error:", err)
		return false
	}

	if currentVersion == version.Dd {
		return false
	}

	currentVersion = version.Dd
	log.Println("riot static: using version " + currentVersion)
	return true
}

func updateChampions() {
	resp, err := http.Get("http://ddragon.leagueoflegends.com/cdn/" + currentVersion + "/data/en_US/champion.json")
	if err != nil {
		log.Println("riot static: failed to get champions:", err)
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("riot static: champion not OK:", resp.Status)
		return
	}

	dec := json.NewDecoder(resp.Body)
	var champions championData
	if err := dec.Decode(&champions); err != nil {
		log.Println("riot static: champion decode error:", err)
		return
	}

	championsMutex.Lock()
	defer championsMutex.Unlock()
	for _, champion := range champions.Data {
		num, err := strconv.Atoi(champion.Key)
		if err != nil {
			log.Println("riot static: champion invalid id:", champion.Key)
			continue
		}
		allChampions[num] = champion
	}

	staticDataMutex.Lock()
	staticDataAvailable = true
	staticDataMutex.Unlock()
}
