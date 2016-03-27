package main

import (
	"github.com/1lann/lol-replay/record"
	"github.com/1lann/lol-replay/recording"
	"github.com/1lann/lol-replay/replay"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
)

type internalRecording struct {
	location  string
	file      *os.File
	rec       *recording.Recording
	temporary bool
	recording bool
}

type internalServer struct {
	replayRouter http.Handler
}

var recordings = make(map[string]*internalRecording)
var recordingsMutex = new(sync.RWMutex)

func isNumber(str string) bool {
	for _, letter := range str {
		if letter < '0' || letter > '9' {
			return false
		}
	}

	return true
}

func retrieve(region, gameId string) *recording.Recording {
	if !record.IsValidPlatform(region) || !isNumber(gameId) {
		return nil
	}

	recordingsMutex.RLock()
	defer recordingsMutex.RUnlock()

	internalRec, found := recordings[region+"_"+gameId]
	if !found {
		return nil
	}

	if internalRec.temporary {
		return nil
	}

	return internalRec.rec
}

func (s *internalServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, replay.PathHeader) {
		s.replayRouter.ServeHTTP(w, r)
		return
	}

	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("insert home page here"))
		return
	}

	// if asset, found := staticAssets[r.URL.Path]; found {
	// 	asset.ServeHTTP(w, r)
	// 	return
	// }

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 page not found"))
}

func main() {
	configLocation := "config.json"
	if len(os.Args) > 1 {
		configLocation = os.Args[1]
	}

	config := readConfiguration(configLocation)

	dir, err := ioutil.ReadDir(config.RecordingsDirectory)
	if os.IsNotExist(err) {
		os.Mkdir(config.RecordingsDirectory, 0755)
	} else if err != nil {
		log.Fatal(err)
		return
	} else {
		loadRecordings(dir, config.RecordingsDirectory)
	}

	internal := &internalServer{replay.Router(retrieve)}

	go monitorPlayers(config)

	log.Fatal(http.ListenAndServe(config.BindAddress, internal))
}

func loadRecordings(dir []os.FileInfo, dirName string) {
	for _, fileInfo := range dir {
		if fileInfo.IsDir() {
			continue
		}

		if path.Ext(fileInfo.Name()) != ".glr" {
			continue
		}

		file, err := os.OpenFile(dirName+"/"+fileInfo.Name(), os.O_RDWR, 0666)
		if err != nil {
			log.Println("failed to open "+fileInfo.Name()+":", err)
			continue
		}

		rec, err := recording.NewRecording(file)
		if err != nil {
			log.Println("failed to read recording "+fileInfo.Name()+":", err)
			continue
		}

		internalRec := &internalRecording{
			file:      file,
			location:  dirName + "/" + fileInfo.Name(),
			rec:       rec,
			temporary: false,
			recording: false,
		}

		key := rec.RetrieveGameInfo().Platform + "_" +
			rec.RetrieveGameInfo().GameId

		recordings[key] = internalRec
	}
}
