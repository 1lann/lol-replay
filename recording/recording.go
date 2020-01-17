// Package recording handles the encoding and decoding of recorded games to
// files.
package recording

//go:generate ffjson $GOFILE

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/1lann/countwriter"
	"github.com/pquerna/ffjson/ffjson"
)

// FormatVersion is the version number of the recording format. As of right
// now, recording formats are not forwards or backwards compatible.
const FormatVersion = 8

const versionPosition = -2
const headerSizePosition = -4
const bufferSize = 200000

type segment struct {
	Position int64
	Length   int
}

type recordingHeader struct {
	GameMetadata   segment
	FirstChunkInfo ChunkInfo
	LastChunkInfo  ChunkInfo
	KeyFrameMap    map[int]segment
	ChunkMap       map[int]segment
	Info           GameInfo
	UserMetadata   segment
	IsComplete     bool
	LastWriteTime  time.Time
}

// GameInfo represents meta information for a game required to play it back
// at a stored recording level.
type GameInfo struct {
	Platform      string
	Version       string
	GameID        string
	EncryptionKey string
	RecordTime    time.Time
}

// Recording manages the reading and writing of recording data to an
// io.ReadWriteSeeker such as an *os.File
type Recording struct {
	file     io.ReadWriteSeeker
	position int64
	header   recordingHeader
	mutex    *sync.Mutex
}

// ChunkInfo is used to store and decode relevant chunk information from the
// recorded game. It is used to store the
type ChunkInfo struct {
	CurrentChunk    int `json:"chunkId"`
	AvailableSince  int `json:"availableSince"`
	NextUpdate      int `json:"nextAvailableChunk"`
	CurrentKeyFrame int `json:"keyFrameId"`
	NextChunk       int `json:"nextChunkId"`
	EndStartupChunk int `json:"endStartupChunkId"`
	StartGameChunk  int `json:"startGameChunkId"`
	EndGameChunk    int `json:"endGameChunkId"`
	Duration        int `json:"duration"`
}

// Error variables to check what errors have occurred.
var (
	ErrMissingData         = errors.New("recording: missing data")
	ErrCannotModify        = errors.New("recording: cannot modify read-only data")
	ErrCorruptRecording    = errors.New("recording: corrupt recording")
	ErrIncompatibleVersion = errors.New("recording: incompatible or invalid format version")
	ErrHeaderTooLarge      = errors.New("recording: header is too large")
)

var bufferPool *sync.Pool

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
		// if pathErr, ok := err.(*os.PathError); ok {
		// 	if pathErr.Err == syscall.EINVAL {
		// 		// Cannot seek to this point, declare data as missing
		// 		return 0, 0, ErrMissingData
		// 	}
		// }
		return 0, 0, ErrMissingData
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

	r.position = int64(pos)

	// Read the header data
	if _, err = r.file.Seek(-(int64(size))-4, 2); err != nil {
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
	r.header.LastWriteTime = time.Now()

	if _, err := r.file.Seek(int64(r.position), 0); err != nil {
		return err
	}

	writer := countwriter.NewWriter(r.file)
	gob.NewEncoder(writer).Encode(r.header)

	if writer.Count() > 65535 {
		return ErrHeaderTooLarge
	}

	// Write preamble headers
	// The size of the header
	if err := binary.Write(r.file, binary.LittleEndian,
		uint16(writer.Count())); err != nil {
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
	r.position += int64(written)
	return segment{writtenPosition, written}, err
}

func (r *Recording) writeToStack(rd io.Reader) (segment, error) {
	if _, err := r.file.Seek(int64(r.position), 0); err != nil {
		return segment{}, err
	}

	writtenPosition := r.position

	written, err := io.Copy(r.file, rd)
	r.position += written
	return segment{writtenPosition, int(written)}, err
}

// WriteTo encodes the ChunkInfo as JSON and writes it to a writer.
func (c ChunkInfo) WriteTo(w io.Writer) (int64, error) {
	cw := countwriter.NewWriter(w)
	encoder := ffjson.NewEncoder(cw)
	err := encoder.EncodeFast(&c)
	return int64(cw.Count()), err
}

func init() {
	bufferPool = new(sync.Pool)
	bufferPool.New = func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, bufferSize))
	}

	gob.Register(recordingHeader{})
}
