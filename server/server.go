package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/1lann/lol-replay/record"
	"github.com/1lann/lol-replay/recording"
	"github.com/1lann/lol-replay/replay"
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

type byTime []*internalRecording

func (d byTime) Len() int      { return len(d) }
func (d byTime) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d byTime) Less(i, j int) bool {
	return d[i].rec.RetrieveGameInfo().RecordTime.Before(d[j].rec.RetrieveGameInfo().RecordTime)
}

var sortedRecordings []*internalRecording
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

func retrieve(region, gameID string) *recording.Recording {
	if !record.IsValidPlatform(region) || !isNumber(gameID) {
		return nil
	}

	recordingsMutex.RLock()
	defer recordingsMutex.RUnlock()

	internalRec, found := recordings[region+"_"+gameID]
	if !found {
		return nil
	}

	if internalRec.temporary {
		return nil
	}

	return internalRec.rec
}

func (s *internalServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			log.Printf("serve HTTP panic: %s: %s", e, debug.Stack())

			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Sorry, an error occurred while " +
				"processing your request!"))
		}
	}()

	if strings.HasPrefix(r.URL.Path, replay.PathHeader) {
		s.replayRouter.ServeHTTP(w, r)
		return
	}

	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api") {
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")

		err := r.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid URL query"}`))
			return
		}

		num, err := strconv.Atoi(r.Form.Get("n"))
		if err != nil || num <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid n value"}`))
			return
		}

		skip, _ := strconv.Atoi(r.Form.Get("skip"))
		if skip < 0 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid skip value"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		writeLastGames(skip, num, r, w)
		return
	}

	if asset, found := staticAssets[r.URL.Path]; found {
		asset.ServeHTTP(w, r)
		return
	}

	serveView(w, r)
	return
}

func main() {
	configLocation := "config.json"
	if len(os.Args) > 1 {
		configLocation = os.Args[1]
	}

	readConfiguration(configLocation)

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

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c

		log.Println("stopping gracefully...")
		recordingsMutex.Lock()

		// Lock recordings to safely close them
		wg := new(sync.WaitGroup)
		for _, internalRec := range recordings {
			wg.Add(1)
			go func(internalRec *internalRecording) {
				internalRec.rec.Lock()
				internalRec.file.Close()
				wg.Done()
			}(internalRec)
		}

		wg.Wait()
		os.Exit(0)
	}()

	go maintainStaticData()
	cleanUp()
	go monitorPlayers()

	log.Fatal(http.ListenAndServe(config.BindAddress, internal))
}

func loadRecordings(dir []os.FileInfo, dirName string) {
	for _, fileInfo := range dir {
		if fileInfo.IsDir() {
			continue
		}

		filename := path.Base(fileInfo.Name())

		if path.Ext(filename) != ".glr" {
			continue
		}

		file, err := os.OpenFile(dirName+"/"+filename, os.O_RDWR, 0666)
		if err != nil {
			log.Println("failed to open "+filename+":", err)
			continue
		}

		rec, err := recording.NewRecording(file)
		if err != nil {
			log.Println("failed to read recording "+filename+":", err)
			file.Close()
			continue
		}

		if !rec.HasGameMetadata() {
			file.Close()
			log.Println("deleting empty recording: " + filename)
			if err := os.Remove(dirName + "/" + filename); err != nil {
				log.Println("failed to delete empty recording:", err)
			}
			continue
		}

		internalRec := &internalRecording{
			file:      file,
			location:  dirName + "/" + filename,
			rec:       rec,
			temporary: false,
			recording: false,
		}

		key := rec.RetrieveGameInfo().Platform + "_" +
			rec.RetrieveGameInfo().GameID

		recordings[key] = internalRec
		sortedRecordings = append(sortedRecordings, internalRec)
	}

	sort.Sort(byTime(sortedRecordings))
}
