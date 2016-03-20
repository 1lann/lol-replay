package recording

import (
	"io"
)

// StoreGameInfo stores the game's basic information to the file.
func (r *Recording) StoreGameInfo(info GameInfo) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.header.Info = info
	return r.writeHeader()
}

// StoreGameMetadata stores the game metadata to the file.
func (r *Recording) StoreGameMetadata(metadata []byte) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.header.Metadata.Length > 0 {
		// Existing metadata
		return ErrCannotModify
	}

	seg, err := r.storeInStack(metadata)
	if err != nil {
		return err
	}

	r.header.Metadata = seg
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
func (r *Recording) StoreChunk(num int, data []byte) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, found := r.header.ChunkMap[num]; found {
		return ErrCannotModify
	}

	seg, err := r.storeInStack(data)
	if err != nil {
		return err
	}

	r.header.ChunkMap[num] = seg
	return r.writeHeader()
}

// StoreKeyFrame stores the key frame data for a key frame number. If the key
// frame already exists in the recording, ErrCannotModify will be returned.
func (r *Recording) StoreKeyFrame(num int, data []byte) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, found := r.header.KeyFrameMap[num]; found {
		return ErrCannotModify
	}

	seg, err := r.storeInStack(data)
	if err != nil {
		return err
	}

	r.header.KeyFrameMap[num] = seg
	return r.writeHeader()
}

func (r *Recording) HasChunk(num int) bool {
	_, found := r.header.ChunkMap[num]
	return found
}

func (r *Recording) HasKeyFrame(num int) bool {
	_, found := r.header.KeyFrameMap[num]
	return found
}

// RetrieveGameInfo retrieves the recorded game's basic information.
func (r *Recording) RetrieveGameInfo() GameInfo {
	return r.header.Info
}

// RetrieveGameMetadata retrieves the recorded game metadata.
func (r *Recording) RetrieveGameMetadata() ([]byte, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.header.Metadata.Length <= 0 {
		return nil, ErrMissingData
	}

	r.file.Seek(int64(r.header.Metadata.Position), 0)
	metadata := make([]byte, r.header.Metadata.Length)
	_, err := io.ReadFull(r.file, metadata)

	return metadata, err
}

// RetrieveFirstChunkID retrieves the game's first chunk ID.
func (r *Recording) RetrieveFirstChunkID() int {
	return r.header.FirstChunkID
}

// RetrieveFirstChunkInfo retrieves the chunk info that should be returned
// first to the client.
func (r *Recording) RetrieveFirstChunkInfo() ChunkInfo {
	return r.header.FirstChunkInfo
}

// StoreLastChunkInfo retrieves the chunk info that should be returned
// after FirstChunkInfo.
func (r *Recording) RetrieveLastChunkInfo() ChunkInfo {
	return r.header.LastChunkInfo
}

// RetrieveChunk retrieves the chunk data for a chunk ID. If the chunk ID does
// not exist, ErrMissingData will be returned.
func (r *Recording) RetrieveChunk(num int) ([]byte, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	seg, found := r.header.ChunkMap[num]
	if !found {
		return nil, ErrMissingData
	}

	r.file.Seek(int64(seg.Position), 0)
	chunkData := make([]byte, seg.Length)
	_, err := io.ReadFull(r.file, chunkData)

	return chunkData, err
}

// RetrieveKeyFrame retrieves the key frame for a key frame number. If the key
// frame does not exist, ErrMissingData will be returned.
func (r *Recording) RetrieveKeyFrame(num int) ([]byte, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	seg, found := r.header.KeyFrameMap[num]
	if !found {
		return nil, ErrMissingData
	}

	r.file.Seek(int64(seg.Position), 0)
	keyFrameData := make([]byte, seg.Length)
	_, err := io.ReadFull(r.file, keyFrameData)

	return keyFrameData, err
}
