package record

import (
	"encoding/json"
	"errors"
	"github.com/1lann/lol-replay/recording"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const retryWaitDuration = time.Second * 5

var ErrNotFound = errors.New("not found")
var ErrUnknownPlatform = errors.New("unknown platform")

func requestOnceURL(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}

		return nil, errors.New(resp.Status)
	}

	return resp.Body, nil
}

func requestURL(url string) (io.ReadCloser, error) {
	var lastError error

	for i := 0; i < 3; i++ {
		var reader io.ReadCloser
		reader, lastError = requestOnceURL(url)
		if lastError == ErrNotFound {
			return nil, ErrNotFound
		} else if lastError != nil {
			time.Sleep(retryWaitDuration)
			continue
		}

		return reader, nil
	}

	return nil, newError("request URL", lastError)
}

func requestURLWriteTo(url string, w io.Writer) (int, error) {
	reader, err := requestURL(url)
	if err != nil {
		return 0, err
	}

	defer reader.Close()
	num, err := io.Copy(w, reader)
	return int(num), err
}

func requestURLBytes(url string) ([]byte, error) {
	reader, err := requestURL(url)
	if err != nil {
		return nil, err
	}

	defer reader.Close()
	return ioutil.ReadAll(reader)
}

type metadata struct {
	StartupChunk int `json:"endStartupChunkId"`
	LastChunk    int `json:"lastChunkId"`
}

// IsValidPlatform returns whether or not a platform is valid (i.e. has an
// entry in the map of platforms and platform URLs).
func IsValidPlatform(platform string) bool {
	_, found := platformURLs[platform]
	return found
}

// GetPlatformVersion returns the current version of the specified platform.
func GetPlatformVersion(platform string) (string, error) {
	url, found := platformURLs[platform]
	if !found {
		return "", newError("get platform version", ErrUnknownPlatform)
	}

	resp, err := requestURLBytes(url + "/observer-mode/rest/consumer/version")
	if err != nil {
		return "", newError("get platform version", err)
	}

	return string(resp), nil
}

func (r *recorder) retrieveMetadata() (metadata, []byte, error) {
	resp, err := requestURLBytes(r.platformURL +
		"/observer-mode/rest/consumer/getGameMetaData/" + r.platform +
		"/" + r.gameId + "/0/token")
	if err != nil {
		return metadata{}, nil, newError("metadata", err)
	}

	result := metadata{}
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return metadata{}, nil, newError("metadata", err)
	}

	return result, resp, nil
}

func (r *recorder) storeChunk(frame int) error {
	if frame <= 0 {
		return nil
	}

	if r.recording.HasChunk(frame) {
		return nil
	}

	resp, err := requestURL(r.platformURL +
		"/observer-mode/rest/consumer/getGameDataChunk/" + r.platform + "/" +
		r.gameId + "/" + strconv.Itoa(frame) + "/token")
	if err != nil {
		return newError("chunk", err)
	}

	if err := r.recording.StoreChunk(frame, resp); err != nil {
		return newError("chunk", err)
	}
	return nil
}

func (r *recorder) storeKeyFrame(frame int) error {
	if frame == 0 {
		return nil
	}

	if r.recording.HasKeyFrame(frame) {
		return nil
	}

	resp, err := requestURL(r.platformURL +
		"/observer-mode/rest/consumer/getKeyFrame/" + r.platform + "/" +
		r.gameId + "/" + strconv.Itoa(frame) + "/token")
	if err != nil {
		return newError("key frame", err)
	}

	if err := r.recording.StoreKeyFrame(frame, resp); err != nil {
		return newError("key frame", err)
	}
	return nil
}

func (r *recorder) retrieveLastChunkInfo() (recording.ChunkInfo, error) {
	resp, err := requestURLBytes(r.platformURL +
		"/observer-mode/rest/consumer/getLastChunkInfo/" + r.platform + "/" +
		r.gameId + "/0/token")
	if err != nil {
		return recording.ChunkInfo{}, newError("last chunk info", err)
	}

	var result recording.ChunkInfo
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return recording.ChunkInfo{}, newError("last chunk info", err)
	}

	return result, nil
}
