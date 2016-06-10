package recording

import (
	"bytes"
	"encoding/gob"
	"io"
	"time"
)

// Lock locks the recording to disallow any further reads or writes to the
// recording. This is used to safely close the underlying file without
// corrupting data written to it, or for other purposes to block reads
// and writes.
func (r *Recording) Lock() {
	r.mutex.Lock()
}

// Unlock unlocks the recording that was previously locked to allow reads or
// writes to the recording. Ensure that Unlock is called after Lock is called.
func (r *Recording) Unlock() {
	r.mutex.Unlock()
}

// DeclareComplete declares the recording as a complete recording.
func (r *Recording) DeclareComplete() error {
	if r.header.IsComplete {
		return nil
	}

	r.header.IsComplete = true
	return r.writeHeader()
}

// StoreUserMetadata stores arbitrary data with the recording for convenience.
// Note that the user metadata is read-only, and thus can only be stored once.
// It can be retrieved with RetrieveUserMetadata.
func (r *Recording) StoreUserMetadata(metadata interface{}) error {
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	gob.RegisterName("UserMetadata", metadata)
	if err := gob.NewEncoder(buf).Encode(metadata); err != nil {
		return err
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.header.UserMetadata.Length > 0 {
		return ErrCannotModify
	}

	seg, err := r.writeToStack(buf)
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
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if _, err := buf.ReadFrom(rd); err != nil {
		return err
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.header.GameMetadata.Length > 0 {
		return ErrCannotModify
	}

	seg, err := r.writeToStack(buf)
	if err != nil {
		return err
	}

	r.header.GameMetadata = seg
	r.header.Info.RecordTime = time.Now()

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
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if _, err := buf.ReadFrom(rd); err != nil {
		return err
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, found := r.header.ChunkMap[num]; found {
		return ErrCannotModify
	}

	seg, err := r.writeToStack(buf)
	if err != nil {
		return err
	}

	r.header.ChunkMap[num] = seg
	return r.writeHeader()
}

// StoreKeyFrame stores the keyframe data for a keyframe number. If the key
// frame already exists in the recording, ErrCannotModify will be returned.
func (r *Recording) StoreKeyFrame(num int, rd io.Reader) error {
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if _, err := buf.ReadFrom(rd); err != nil {
		return err
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, found := r.header.KeyFrameMap[num]; found {
		return ErrCannotModify
	}

	seg, err := r.writeToStack(buf)
	if err != nil {
		return err
	}

	r.header.KeyFrameMap[num] = seg
	return r.writeHeader()
}
