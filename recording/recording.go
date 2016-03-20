// Package recording handles the encoding and decoding of recorded games to
// files.
package recording

import (
	"bufio"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"io"
	"os"
	"sync"
	"syscall"
)

const bufferSize = 65535

// FormatVersion is the version number of the recording format. As of right
// now, recording formats are not forwards or backwards compatible.
const FormatVersion = 2

const versionPosition = -2
const headerSizePosition = -4

type segment struct {
	Position int
	Length   int
}

type recordingHeader struct {
	Metadata       segment
	FirstChunkInfo ChunkInfo
	LastChunkInfo  ChunkInfo
	FirstChunkID   int
	KeyFrameMap    map[int]segment
	ChunkMap       map[int]segment
	Info           GameInfo
}

type GameInfo struct {
	Platform      string
	Version       string
	GameId        string
	EncryptionKey string
}

// Recording manages the reading and writing of recording data to an
// io.ReadWriteSeeker such as an *os.File
type Recording struct {
	file     io.ReadWriteSeeker
	position int
	header   recordingHeader
	mutex    *sync.Mutex
}

// ChunkInfo is used to store and decode relevant chunk information from the
// recorded game. It is used to store the
type ChunkInfo struct {
	NextChunk       int `json:"nextChunkId"`
	CurrentChunk    int `json:"chunkId"`
	NextUpdate      int `json:"nextAvailableChunk"`
	StartGameChunk  int `json:"startGameChunkId"`
	CurrentKeyFrame int `json:"keyFrameId"`
	EndGameChunk    int `json:"endGameChunkId"`
	AvailableSince  int `json:"availableSince"`
	Duration        int `json:"duration"`
	EndStartupChunk int `json:"endStartupChunkId"`
}

var (
	ErrMissingData         = errors.New("recording: missing data")
	ErrCannotModify        = errors.New("recording: cannot modify read-only data")
	ErrCorruptRecording    = errors.New("recording: corrupt recording")
	ErrIncompatibleVersion = errors.New("recording: incompatible format version")
)

// NewRecording creates a new recording for writing to, or reads an existing
// recording to read from using the io.ReadWriteSeeker, such as an *os.File.
func NewRecording(file io.ReadWriteSeeker) (*Recording, error) {
	recording := &Recording{
		file:     file,
		position: 0,
		mutex:    new(sync.Mutex),
		header: recordingHeader{
			ChunkMap:    make(map[int]segment),
			KeyFrameMap: make(map[int]segment),
		},
	}

	if err := recording.readHeader(); err != nil {
		if err != ErrMissingData {
			return nil, err
		}
	}

	return recording, nil
}

func (r *Recording) readHeaderUnit16(offset int) (uint16, int, error) {
	pos, err := r.file.Seek(int64(offset), 2)
	if err != nil {
		if pathErr, ok := err.(*os.PathError); ok {
			if pathErr.Err == syscall.EINVAL {
				// Cannot seek to this point, declare data as missing
				return 0, 0, ErrMissingData
			}
		}

		return 0, 0, err
	}

	var number uint16
	err = binary.Read(r.file, binary.LittleEndian, &number)
	if err != nil {
		return 0, int(pos), err
	}

	return number, int(pos), nil
}

func (r *Recording) readHeader() error {
	// Read preamble headers
	// Read the version
	version, _, err := r.readHeaderUnit16(versionPosition)
	if err != nil {
		return err
	}

	if version != FormatVersion {
		return ErrIncompatibleVersion
	}

	// Read the header size
	size, pos, err := r.readHeaderUnit16(headerSizePosition)
	if err != nil {
		return err
	}

	r.position = pos

	// Read the header data
	if _, err := r.file.Seek(-(int64(size))-4, 2); err != nil {
		if pathErr, ok := err.(*os.PathError); ok {
			if pathErr.Err == syscall.EINVAL {
				// Header size is too long, recording is corrupt
				return ErrCorruptRecording
			}
		}

		return err
	}

	reader := io.LimitReader(r.file, int64(size))
	decoder := gob.NewDecoder(reader)
	err = decoder.Decode(&r.header)
	if err != nil {
		return ErrCorruptRecording
	}

	return nil
}

func (r *Recording) writeHeader() error {
	r.file.Seek(int64(r.position), 0)
	writer := bufio.NewWriterSize(r.file, bufferSize)
	gob.NewEncoder(writer).Encode(r.header)

	size := writer.Buffered()
	err := writer.Flush()
	if err != nil {
		return err
	}

	// Write preamble headers
	// The size of the header
	if err := binary.Write(r.file, binary.LittleEndian,
		uint16(size)); err != nil {
		return err
	}

	// The version of the recording format
	if err := binary.Write(r.file, binary.LittleEndian,
		uint16(FormatVersion)); err != nil {
		return err
	}

	return nil
}

func (r *Recording) storeInStack(data []byte) (segment, error) {
	if _, err := r.file.Seek(int64(r.position), 0); err != nil {
		return segment{}, err
	}

	writtenPosition := r.position

	written, err := r.file.Write(data)
	r.position += written
	return segment{writtenPosition, written}, err
}

func init() {
	gob.Register(recordingHeader{})
}
