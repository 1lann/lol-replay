package main

import (
	"fmt"
	"github.com/1lann/lol-replay/recording"
	"os"
)

func main() {
	file, err := os.OpenFile("KR_2361669561.glr", os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}

	defer file.Close()

	rec, err := recording.NewRecording(file)
	if err != nil {
		panic(err)
	}

	info := rec.RetrieveFirstChunkInfo()

	fmt.Println("old:", info.EndGameChunk)
	fmt.Println(info)

	info.EndStartupChunk = info.EndStartupChunk - 1

	fmt.Println(info.CurrentChunk)

	err = rec.StoreFirstChunkInfo(info)
	if err != nil {
		panic(err)
	}
}
