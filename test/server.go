package main

import (
	"fmt"
	"github.com/1lann/lol-replay/record"
	"os"
)

func main() {
	file, err := os.OpenFile("test.glrf", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}

	err = record.Record("KR", "2353549641", "Z/ywgwW1ZoXNkKucp/wleO14xX533IQ1", file)
	if err != nil {
		fmt.Println(err)
	}
}
