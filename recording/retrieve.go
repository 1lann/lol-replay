package recording

import (
	"bytes"
	"encoding/gob"
	"io"
	"time"
)

// HasChunk returns whether or not the specified chunk ID already exists in
// the recording or not.
func (r *Recording) HasChunk(num int) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, found := r.header.ChunkMap[num]
	return found
}

// HasKeyFrame returns whether or not the specified keyframe already exists in
// the recording or not.
func (r *Recording) HasKeyFrame(num int) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, found := r.header.KeyFrameMap[num]
	return found
}

// HasGameMetadata returns whether or not the metadata of the game has already
// been written to the recording or not.
func (r *Recording) HasGameMetadata() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.header.GameMetadata.Length > 0
}

// HasUserMetadata returns whether or not the user metadata has already been
// written to the recording or not.
func (r *Recording) HasUserMetadata() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.header.UserMetadata.Length > 0
}

// RetrieveUserMetadata retrieves the arbitrary user data stored by
// StoreUserMetadata into metadata.
func (r *Recording) RetrieveUserMetadata(metadata interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.header.UserMetadata.Length <= 0 {
		return ErrMissingData
	}

	gob.RegisterName("UserMetadata", metadata)

	if _, err := r.file.Seek(int64(r.header.UserMetadata.Position), 0); err != nil {
		return err
	}

	dec := gob.NewDecoder(io.LimitReader(r.file, int64(r.header.UserMetadata.Length)))
	return dec.Decode(metadata)
}

// RetrieveGameInfo retrieves the recorded game's basic information.
func (r *Recording) RetrieveGameInfo() GameInfo {
	return r.header.Info
}

// IsComplete returns whether or not the recording has been declared as
// being complete or not.
func (r *Recording) IsComplete() bool {
	return r.header.IsComplete
}

// LastWriteTime returns the last time data was written to the recording.
func (r *Recording) LastWriteTime() time.Time {
	return r.header.LastWriteTime
}

// RetrieveGameMetadataTo retrieves the recorded game metadata into w.
// The number of bytes written to w and any errors that have occurred are
// returned.
func (r *Recording) RetrieveGameMetadataTo(w io.Writer) (int, error) {
	r.mutex.Lock()

	if r.header.GameMetadata.Length <= 0 {
		r.mutex.Unlock()
		return 0, ErrMissingData
	}

	if _, err := r.file.Seek(int64(r.header.GameMetadata.Position), 0); err != nil {
		r.mutex.Unlock()
		return 0, err
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if _, err := io.CopyN(buf, r.file, int64(r.header.GameMetadata.Length)); err != nil {
		r.mutex.Unlock()
		return 0, err
	}
	r.mutex.Unlock()

	written, err := buf.WriteTo(w)
	return int(written), err

}

// RetrieveFirstChunkInfo retrieves the chunk info that should be returned
// first to the client.
func (r *Recording) RetrieveFirstChunkInfo() ChunkInfo {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.header.FirstChunkInfo
}

// RetrieveLastChunkInfo retrieves the chunk info that should be returned
// after FirstChunkInfo.
func (r *Recording) RetrieveLastChunkInfo() ChunkInfo {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.header.LastChunkInfo
}

// RetrieveChunkTo retrieves the chunk data for a chunk ID into w. The number
// of bytes written to w and any errors that have occurred are returned.
// If the chunk ID does not exist, ErrMissingData will be returned.
func (r *Recording) RetrieveChunkTo(num int, w io.Writer) (int, error) {
	r.mutex.Lock()

	seg, found := r.header.ChunkMap[num]
	if !found {
		r.mutex.Unlock()
		return 0, ErrMissingData
	}

	if _, err := r.file.Seek(int64(seg.Position), 0); err != nil {
		r.mutex.Unlock()
		return 0, err
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if _, err := io.CopyN(buf, r.file, int64(seg.Length)); err != nil {
		r.mutex.Unlock()
		return 0, err
	}
	r.mutex.Unlock()

	written, err := buf.WriteTo(w)
	return int(written), err
}

// RetrieveKeyFrameTo retrieves the keyframe data into w. The number
// of bytes written to w and any errors that have occurred are returned.
// If the chunk ID does not exist, ErrMissingData will be returned.
func (r *Recording) RetrieveKeyFrameTo(num int, w io.Writer) (int, error) {
	r.mutex.Lock()

	seg, found := r.header.KeyFrameMap[num]
	if !found {
		r.mutex.Unlock()
		return 0, ErrMissingData
	}

	if _, err := r.file.Seek(int64(seg.Position), 0); err != nil {
		r.mutex.Unlock()
		return 0, err
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if _, err := io.CopyN(buf, r.file, int64(seg.Length)); err != nil {
		r.mutex.Unlock()
		return 0, err
	}
	r.mutex.Unlock()

	written, err := buf.WriteTo(w)
	return int(written), err
}
