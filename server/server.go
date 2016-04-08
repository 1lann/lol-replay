package main

import (
	"github.com/1lann/lol-replay/record"
	"github.com/1lann/lol-replay/recording"
	"github.com/1lann/lol-replay/replay"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"syscall"
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
	defer func() {
		if e := recover(); e != nil {
			log.Printf("serve HTTP panic: %s: %s", e, debug.Stack())

			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Sorry, an error occured while " +
				"processing your request!"))
		}
	}()

	if strings.HasPrefix(r.URL.Path, replay.PathHeader) {
		s.replayRouter.ServeHTTP(w, r)
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
			file.Close()
			continue
		}

		if !rec.HasGameMetadata() {
			file.Close()
			log.Println("deleting empty recording: " + fileInfo.Name())
			if err := os.Remove(dirName + "/" + fileInfo.Name()); err != nil {
				log.Println("failed to delete empty recording:", err)
			}
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
		sortedRecordings = append(sortedRecordings, internalRec)
	}

	sort.Sort(byTime(sortedRecordings))
}
