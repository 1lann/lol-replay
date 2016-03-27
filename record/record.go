// Package record is an interface to record League of Legends games to a file
// using data from the spectator endpoint. Most if not all of the methods
// here will return *RecordingError as the underlying error.
package record

import (
	"bytes"
	"github.com/1lann/lol-replay/recording"
	"time"
)

var platformURLs = map[string]string{
	"NA1":  "http://spectator.na.lol.riotgames.com:80",
	"OC1":  "http://spectator.oc1.lol.riotgames.com:80",
	"EUN1": "http://spectator.eu.lol.riotgames.com:8088",
	"EUW1": "http://spectator.euw1.lol.riotgames.com:80",
	"KR":   "http://spectator.kr.lol.riotgames.com:80",
	"BR1":  "http://spectator.br.lol.riotgames.com:80",
	"LA1":  "http://spectator.la1.lol.riotgames.com:80",
	"LA2":  "http://spectator.la2.lol.riotgames.com:80",
	"RU":   "http://spectator.ru.lol.riotgames.com:80",
	"TR1":  "http://spectator.tr.lol.riotgames.com:80",
	"PBE1": "http://spectator.pbe1.lol.riotgames.com:8088",
}

type recorder struct {
	recording   *recording.Recording
	platformURL string
	platform    string
	gameId      string
}

// Record starts a new recording that writes into a *recording.Recording
// and blocks until the recording ends or an error occurs. Note that partial
// data may be written to the recording, even if the recording was
// unsuccessful. This partial data can probably be played back. ErrNotFound
// enclosed in a *RecordingError is returned if there is no game that can be
// recorded from the provided parameters.
func Record(platform, gameId, encryptionKey string,
	rec *recording.Recording) error {
	url, found := platformURLs[platform]
	if !found {
		return newError("", ErrUnknownPlatform)
	}

	resumption := false
	if rec.HasGameMetadata() {
		resumption = true
	}

	thisRecorder := &recorder{
		platformURL: url,
		recording:   rec,
		platform:    platform,
		gameId:      gameId,
	}

	version, err := GetPlatformVersion(platform)
	if err != nil {
		return err
	}

	if err := thisRecorder.recording.StoreGameInfo(recording.GameInfo{
		Platform:      platform,
		Version:       version,
		GameId:        gameId,
		EncryptionKey: encryptionKey,
		RecordTime:    time.Now(),
	}); err != nil {
		return err
	}

	if err := thisRecorder.waitForFirstChunk(); err != nil {
		return err
	}

	if err := thisRecorder.recordFrames(resumption); err != nil {
		return err
	}

	if !resumption {
		thisRecorder.recording.DeclareComplete()
	}

	return nil
}

func (r *recorder) waitForFirstChunk() error {
	meta, data, err := r.retrieveMetadata()
	if err != nil {
		return err
	}

	if !r.recording.HasGameMetadata() {
		for {
			chunk, err := r.retrieveLastChunkInfo()
			if err != nil {
				return err
			}

			if chunk.CurrentChunk > meta.StartupChunk {
				break
			}

			time.Sleep(time.Duration(chunk.NextUpdate)*time.Millisecond +
				time.Second)
		}

		meta, data, err = r.retrieveMetadata()
		if err != nil {
			return err
		}

		if err := r.recording.StoreGameMetadata(bytes.NewReader(data)); err != nil {
			return err
		}
	}

	// Get the startup frames
	for i := 1; i <= meta.StartupChunk; i++ {
		for {
			chunk, err := r.retrieveLastChunkInfo()
			if err != nil {
				if err.(*RecordingError).Err == ErrNotFound {
					time.Sleep(time.Second * 10)
					continue
				}

				return err
			}

			if i > chunk.CurrentChunk {
				time.Sleep(time.Duration(chunk.NextUpdate)*time.Millisecond +
					time.Second)
				continue
			}

			if err := r.storeChunk(i); err != nil {
				return err
			}
			break
		}
	}

	return nil
}

func (r *recorder) recordFrames(resumption bool) error {
	firstChunkID := 0
	firstKeyFrame := 0
	lastChunkID := 0
	lastKeyFrame := 0

	if resumption {
		// Restore information
		firstChunkInfo := r.recording.RetrieveFirstChunkInfo()
		firstChunkID = firstChunkInfo.CurrentChunk
		firstKeyFrame = firstChunkInfo.CurrentKeyFrame
		lastChunkInfo := r.recording.RetrieveLastChunkInfo()
		lastChunkID = lastChunkInfo.CurrentChunk
		lastKeyFrame = lastChunkInfo.CurrentKeyFrame
	}

	for {
		chunk, err := r.retrieveLastChunkInfo()
		if err != nil {
			return err
		}

		if resumption {
			lastChunkID = chunk.CurrentChunk
			lastKeyFrame = chunk.CurrentKeyFrame

			// Download as much previous data (as fast) as possible
			go func() {
				for i := chunk.CurrentChunk; i >= chunk.StartGameChunk; i-- {
					if err := r.storeChunk(i); err != nil {
						return
					}
				}
			}()

			go func() {
				for i := chunk.CurrentKeyFrame; i >= 1; i-- {
					if err := r.storeKeyFrame(i); err != nil {
						return
					}
				}
			}()

			resumption = false
		}

		if firstChunkID == 0 {
			firstChunkID = chunk.StartGameChunk
			if chunk.CurrentChunk > chunk.StartGameChunk {
				firstChunkID = chunk.CurrentChunk
			}

			firstKeyFrame = 1
			if chunk.CurrentKeyFrame > 0 {
				firstKeyFrame = chunk.CurrentKeyFrame
			}

			lastChunkID = chunk.CurrentChunk
			lastKeyFrame = chunk.CurrentKeyFrame

			if err := r.storeChunk(chunk.CurrentChunk); err != nil {
				return err
			}
			if err := r.storeKeyFrame(chunk.CurrentKeyFrame); err != nil {
				return err
			}
		}

		if chunk.StartGameChunk > firstChunkID {
			firstChunkID = chunk.StartGameChunk
		}

		if chunk.CurrentChunk > lastChunkID {
			for i := lastChunkID + 1; i <= chunk.CurrentChunk; i++ {
				if err := r.storeChunk(i); err != nil {
					return err
				}
			}
		}

		if chunk.NextChunk < chunk.CurrentChunk && chunk.NextChunk > 0 {
			if err := r.storeChunk(chunk.NextChunk); err != nil {
				return err
			}
		}

		if chunk.CurrentKeyFrame > lastKeyFrame {
			for i := lastKeyFrame + 1; i <= chunk.CurrentKeyFrame; i++ {
				if err := r.storeKeyFrame(i); err != nil {
					return err
				}
			}
		}

		if err := r.storeChunkInfo(firstChunkID, firstKeyFrame,
			chunk); err != nil {
			return err
		}

		lastChunkID = chunk.CurrentChunk
		lastKeyFrame = chunk.CurrentKeyFrame

		if chunk.EndGameChunk == chunk.CurrentChunk+1 {
			return nil
		}

		time.Sleep(time.Duration(chunk.NextUpdate)*time.Millisecond +
			time.Second)
	}
}

func (r *recorder) storeChunkInfo(firstChunkID, firstKeyFrame int,
	chunk recording.ChunkInfo) error {
	chunkInfo := recording.ChunkInfo{
		NextChunk:       firstChunkID,
		CurrentChunk:    firstChunkID,
		NextUpdate:      0,
		StartGameChunk:  chunk.StartGameChunk,
		CurrentKeyFrame: firstKeyFrame,
		EndGameChunk:    chunk.CurrentChunk,
		AvailableSince:  0,
		Duration:        30000,
		EndStartupChunk: chunk.EndStartupChunk,
	}

	if err := r.recording.StoreFirstChunkInfo(chunkInfo); err != nil {
		return err
	}

	chunkInfo.NextChunk = chunk.CurrentChunk - 1
	chunkInfo.CurrentChunk = chunk.CurrentChunk
	chunkInfo.CurrentKeyFrame = chunk.CurrentKeyFrame

	if err := r.recording.StoreLastChunkInfo(chunkInfo); err != nil {
		return err
	}

	return nil
}
