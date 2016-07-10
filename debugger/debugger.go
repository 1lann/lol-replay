package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/1lann/lol-replay/recording"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("GLR File Debugger")
		fmt.Println("This utility dumps information for debugging recordings")
		fmt.Println("Usage: " + os.Args[0] + " recording.glr")
		return
	}

	file, err := os.OpenFile(os.Args[1], os.O_RDWR, 0666)
	if err != nil {
		fmt.Println("failed to open file:", err)
		return
	}

	rec, err := recording.NewRecording(file)
	if err != nil {
		fmt.Println("failed to read recording:", err)
		file.Close()
		return
	}

	fmt.Println("--- Recording properties ---")
	fmt.Println("Has game metadata:", rec.HasGameMetadata())
	fmt.Println("Has user metadata:", rec.HasUserMetadata())
	fmt.Println("Is recording complete:", rec.IsComplete())
	fmt.Println("")

	fmt.Println("--- Game information ---")
	gameInfo := rec.RetrieveGameInfo()
	fmt.Println("Platform:", gameInfo.Platform)
	fmt.Println("Game version:", gameInfo.Version)
	fmt.Println("Record time", gameInfo.RecordTime.Format(time.RFC1123Z))
	fmt.Println("Encryption key:", gameInfo.EncryptionKey)
	fmt.Println("")

	buf := new(bytes.Buffer)

	fmt.Println("--- Metadata information ---")
	rec.RetrieveGameMetadataTo(buf)
	fmt.Println(buf.String())

	fmt.Println("--- Chunk information ---")
	buf.Reset()
	rec.RetrieveFirstChunkInfo().WriteTo(buf)
	fmt.Println("First chunk information:")
	fmt.Println(buf.String())
	buf.Reset()
	rec.RetrieveLastChunkInfo().WriteTo(buf)
	fmt.Println("Last chunk information:")
	fmt.Println(buf.String())
	fmt.Println("")

	fmt.Println("--- Chunk and key frame data ---")
	fmt.Println("Chunks found:")
	for i := 0; i < 1000; i++ {
		if rec.HasChunk(i) {
			buf.Reset()
			rec.RetrieveChunkTo(i, buf)
			fmt.Println("    " + strconv.Itoa(i) + ": size: " + strconv.Itoa(buf.Len()))
		}
	}

	fmt.Println("Key frames found:")
	for i := 0; i < 1000; i++ {
		if rec.HasKeyFrame(i) {
			buf.Reset()
			rec.RetrieveKeyFrameTo(i, buf)
			fmt.Println("    " + strconv.Itoa(i) + ": size: " + strconv.Itoa(buf.Len()))
		}
	}
	fmt.Println("")
	fmt.Println("--- End of report ---")
}
