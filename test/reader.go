package main

import (
	"fmt"
	"github.com/1lann/lol-replay/recording"
	"os"
)

func main() {
	file, err := os.Open("test.glr")
	if err != nil {
		panic(err)
	}

	recorded, err := recording.NewRecording(file)
	if err != nil {
		panic(err)
	}

	fmt.Println(recorded.RetrieveGameInfo().Platform)
}
