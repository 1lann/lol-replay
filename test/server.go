package main

import (
	"fmt"
	"github.com/1lann/lol-replay/record"
	"os"
)

func main() {
	file, err := os.OpenFile("test.glr", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}

	err = record.Record("KR", "2358837838", "pkrINn/nZaFnKI6u8I6hk4je02xIMsmq", file)
	if err != nil {
		fmt.Println(err)
	}
}
