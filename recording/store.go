package recording

import (
	"encoding/gob"
	"io"
	"time"
)

// StoreUserMetadata stores arbitary data with the recording for convenience.
// It can be retrieved with RetrieveUserMetadata.
func (r *Recording) StoreUserMetadata(metadata interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.header.UserMetadata.Length > 0 {
		return ErrCannotModify
	}

	gob.RegisterName("UserMetadata", metadata)

	rd, w := io.Pipe()

	go func() {
		if err := gob.NewEncoder(w).Encode(metadata); err != nil {
			w.CloseWithError(err)
		}
		w.Close()
	}()

	seg, err := r.writeToStack(rd)
	if err != nil {
		return err
	}

	r.header.UserMetadata = seg

	return r.writeHeader()
}

// StoreGameInfo stores the game's basic information to the file.
func (r *Recording) StoreGameInfo(info GameInfo) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.header.Info = info
	return r.writeHeader()
}

// StoreGameMetadata stores the game metadata to the file.
func (r *Recording) StoreGameMetadata(rd io.Reader) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.header.GameMetadata.Length > 0 {
		// Existing metadata
		return ErrCannotModify
	}

	seg, err := r.writeToStack(rd)
	if err != nil {
		return err
	}

	r.header.GameMetadata = seg
	r.header.Info.RecordTime = time.Now()

	return r.writeHeader()
}

// StoreFirstChunkID stores the game's first chunk ID.
func (r *Recording) StoreFirstChunkID(num int) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.header.FirstChunkID = num
	return r.writeHeader()
}

// StoreFirstChunkInfo stores the chunk info that should be returned
// first to the client.
func (r *Recording) StoreFirstChunkInfo(chunkInfo ChunkInfo) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.header.FirstChunkInfo = chunkInfo
	return r.writeHeader()
}

// StoreLastChunkInfo stores the chunk info that should be returned
// after FirstChunkInfo.
func (r *Recording) StoreLastChunkInfo(chunkInfo ChunkInfo) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.header.LastChunkInfo = chunkInfo
	return r.writeHeader()
}

// StoreChunk stores the chunk data for a chunk ID. If the chunk ID already
// exists in the recording, ErrCannotModify will be returned.
func (r *Recording) StoreChunk(num int, rd io.Reader) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, found := r.header.ChunkMap[num]; found {
		return ErrCannotModify
	}

	seg, err := r.writeToStack(rd)
	if err != nil {
		return err
	}

	r.header.ChunkMap[num] = seg
	return r.writeHeader()
}

// StoreKeyFrame stores the keyframe data for a keyframe number. If the key
// frame already exists in the recording, ErrCannotModify will be returned.
func (r *Recording) StoreKeyFrame(num int, rd io.Reader) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, found := r.header.KeyFrameMap[num]; found {
		return ErrCannotModify
	}

	seg, err := r.writeToStack(rd)
	if err != nil {
		return err
	}

	r.header.KeyFrameMap[num] = seg
	return r.writeHeader()
}
