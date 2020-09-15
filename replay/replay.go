// Package replay handles the serving of recorded data through a HTTP router.
package replay

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/1lann/lol-replay/record"
	"github.com/1lann/lol-replay/recording"
	"github.com/Clever/leakybucket"
	memorybucket "github.com/Clever/leakybucket/memory"
	"github.com/julienschmidt/httprouter"
)

// PathHeader is the common path header used for requests to the spectator
// endpoint.
const PathHeader = "/observer-mode/rest/consumer"

// A client is identified by a IP/gameID pair.
// We want to identify a client because in order to start a spectating session from the
// beginning, we need to "pretend" that the last available chunk is one of the first chunks.
// Therefore if the client is new, we modify the behaviour of getLastChunkInfo.
type client struct {
	IP     string
	gameID string
}

var bucketStore = memorybucket.New()

// A Retriever provides a recording for a given game ID and region.
// A nil recording should be returned if the recording does not exist.
type Retriever func(region, gameId string) *recording.Recording

type requestHandler struct {
	retriever Retriever
	// Track how often a client has been making requests, using the leaky
	// bucket algorithm so that if the same client spectates again after a while,
	// we consider them a new client.
	newClientBuckets map[client]leakybucket.Bucket
}

type httpWriterPipe struct {
	w           http.ResponseWriter
	contentType string
	hasWritten  bool
}

func newHTTPWriterPipe(w http.ResponseWriter,
	contentType string) *httpWriterPipe {
	return &httpWriterPipe{w: w, contentType: contentType, hasWritten: false}
}

func (p *httpWriterPipe) Write(data []byte) (int, error) {
	if !p.hasWritten {
		p.w.Header().Set("Content-Type", p.contentType)
		p.w.WriteHeader(http.StatusOK)
		p.hasWritten = true
	}

	return p.w.Write(data)
}

func (p *httpWriterPipe) HasWritten() bool {
	return p.hasWritten
}

func (rh requestHandler) version(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	version, err := record.GetPlatformVersion("OC1")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("version unavailable"))
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(version))
	return
}

func (rh requestHandler) getGameMetadata(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	rec := rh.retriever(ps.ByName("region"), ps.ByName("id"))
	if rec == nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("game not found"))
		return
	}

	pipe := newHTTPWriterPipe(w, "application/json")
	_, err := rec.RetrieveGameMetadataTo(pipe)
	if err != nil {
		if pipe.HasWritten() {
			log.Println("getGameMetadata silent error:", err)
			return
		}

		if err == recording.ErrMissingData {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("metadata not found"))
			return
		}

		log.Println("getGameMetadata error:", err)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
		return
	}
}

func (rh requestHandler) getLastChunkInfo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	rec := rh.retriever(ps.ByName("region"), ps.ByName("id"))
	if rec == nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("game not found"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Identify the client by the IP/gameID tuple
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	c := client{
		IP:     ip,
		gameID: ps.ByName("id"),
	}

	if rh.newClientBuckets[c] == nil {
		// A normal spectator client should make request to this endpoint once every 10 seconds.
		// Therefore, we use the more conservative number of 3 here, meaning that a client is
		// considered new if it hasn't made 3 requests in the last minute.
		bucket, err := bucketStore.Create(
			fmt.Sprintf("%s-%s", r.RemoteAddr, ps.ByName("id")),
			3,
			time.Minute,
		)
		if err != nil {
			rec.RetrieveLastChunkInfo().WriteTo(w)
			return
		}
		rh.newClientBuckets[c] = bucket
	}

	// We try to figure out if the client is a new or not.  If the client is new, we "pretend"
	// that the last available chunk is one of the first few chunks, so that the spectator client
	// would start playing from the beginning.  Otherwise, we return the real last available chunk.
	_, err := rh.newClientBuckets[c].Add(1)
	if err != nil {
		rec.RetrieveLastChunkInfo().WriteTo(w)
	} else {
		rec.RetrieveFirstChunkInfo().WriteTo(w)
	}
}

func (rh requestHandler) getGameDataChunk(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	rec := rh.retriever(ps.ByName("region"), ps.ByName("id"))
	if rec == nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("game not found"))
		return
	}

	chunk, err := strconv.Atoi(ps.ByName("chunk"))
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid chunk number"))
		return
	}

	pipe := newHTTPWriterPipe(w, "application/octet-stream")
	_, err = rec.RetrieveChunkTo(chunk, pipe)
	if err != nil {
		if pipe.HasWritten() {
			log.Println("getGameDataChunk silent error:", err)
			return
		}

		if err == recording.ErrMissingData {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("chunk not found"))
			return
		}

		log.Println("getGameDataChunk error:", err)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
		return
	}
}

func (rh requestHandler) getKeyFrame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	rec := rh.retriever(ps.ByName("region"), ps.ByName("id"))
	if rec == nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("game not found"))
		return
	}

	frame, err := strconv.Atoi(ps.ByName("frame"))
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid keyframe number"))
		return
	}

	pipe := newHTTPWriterPipe(w, "application/octet-stream")
	_, err = rec.RetrieveKeyFrameTo(frame, pipe)
	if err != nil {
		if pipe.HasWritten() {
			log.Println("getKeyFrame silent error:", err)
			return
		}

		if err == recording.ErrMissingData {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("keyframe not found"))
			return
		}

		log.Println("getKeyFrame error:", err)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
		return
	}
}

// Router returns a http.Handler that handles requests for recorded data.
func Router(retriever Retriever) http.Handler {
	handler := requestHandler{
		retriever:        retriever,
		newClientBuckets: make(map[client]leakybucket.Bucket),
	}

	router := httprouter.New()
	router.GET(PathHeader+"/version", handler.version)
	router.GET(PathHeader+"/getGameMetaData/:region/:id/*ignore",
		handler.getGameMetadata)
	router.GET(PathHeader+"/getLastChunkInfo/:region/:id/:end/*ignore",
		handler.getLastChunkInfo)
	router.GET(PathHeader+"/getGameDataChunk/:region/:id/:chunk/*ignore",
		handler.getGameDataChunk)
	router.GET(PathHeader+"/getKeyFrame/:region/:id/:frame/*ignore",
		handler.getKeyFrame)

	return router
}
