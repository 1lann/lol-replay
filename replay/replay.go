// Package replays handles the serving of recorded data through a HTTP router.
package replay

import (
	"github.com/1lann/lol-replay/record"
	"github.com/1lann/lol-replay/recording"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"strconv"
)

// PathHeader is the common path header used for requests to the spectator
// endpoint.
const PathHeader = "/observer-mode/rest/consumer"

// A ReplayRetriever provides a recording for a given game ID and region.
// A nil recording should be returned if the recording does not exist.
type ReplayRetriever func(region, gameId string) *recording.Recording

type requestHandler struct {
	retriever ReplayRetriever
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
	if ps.ByName("end") == "0" {
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

func Router(retriever ReplayRetriever) http.Handler {
	handler := requestHandler{retriever}

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
