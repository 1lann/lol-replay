package controllers

import (
	"bytes"
	"compress/gzip"
	// "encoding/base64"
	"github.com/revel/revel"
	"golang.org/x/crypto/blowfish"
	"io/ioutil"
	"math"
	"net/http"
	"replay/app/models/history"
	"replay/app/models/record"
	"replay/app/models/replay"
	"time"
)

const testKey = "3BRxZaqrZVF+EL91GM9H+x1KBirpI4Ah"

func decryptAndPrint(data []byte) {
	// encoder := base64.StdEncoding
	// key, _ := encoder.DecodeString(testKey)

	revel.INFO.Println("LENGTH:", len(testKey))

	cipher, err := blowfish.NewCipher([]byte(testKey))
	if err != nil {
		revel.ERROR.Println("Could not create cipher with key!")
		revel.ERROR.Println(err)
		return
	}

	numChunks := int(math.Ceil(float64(len(data)) / float64(8)))

	output := []byte{}

	for i := 0; i < numChunks; i++ {
		start := i * 8
		end := (i + 1) * 8
		srcChunk := data[start:end]
		dstChunk := []byte{0, 0, 0, 0, 0, 0, 0, 0}

		cipher.Decrypt(dstChunk, srcChunk)

		output = append(output, dstChunk...)
	}

	byteData := bytes.NewBuffer(output)

	reader, err := gzip.NewReader(byteData)
	if err != nil {
		revel.ERROR.Println(err)
		return
	}
	defer reader.Close()

	revel.INFO.Println(ioutil.ReadAll(reader))
}

type TextResponse []byte

func (r TextResponse) Apply(req *revel.Request, resp *revel.Response) {
	resp.WriteHeader(http.StatusOK, "application/json")
	resp.Out.Write(r)
}

type OctetResponse []byte

func (r OctetResponse) Apply(req *revel.Request, resp *revel.Response) {
	resp.WriteHeader(http.StatusOK, "application/octet-stream")
	resp.Out.Write(r)
}

type App struct {
	*revel.Controller
}

func (c App) Index() revel.Result {
	c.RenderArgs["games"] = history.GameList()
	return c.Render()
}

func (c App) Record(region, gameId, encryptionKey string) revel.Result {
	record.Record(region, gameId, encryptionKey)
	return c.RenderText("ok.")
}

func (c App) Version() revel.Result {
	return c.RenderText(record.GetVersion())
}

func (c App) Metadata(region, id string) revel.Result {
	resp, ok := replay.GetMetadata(region, id)
	if !ok {
		return c.NotFound("")
	}

	return TextResponse(resp)
}

func (c App) LastChunkInfo(region, id, end string) revel.Result {
	time.Sleep(time.Second)
	resp, ok := replay.GetLastChunkInfo(region, id, end)
	if !ok {
		return c.NotFound("")
	}

	return TextResponse(resp)
}

func (c App) DataChunk(region, id, frame string) revel.Result {
	resp, ok := replay.GetGameDataChunk(region, id, frame)
	if !ok {
		return c.NotFound("")
	}

	decryptAndPrint(resp)

	return OctetResponse(resp)
}

func (c App) KeyFrame(region, id, frame string) revel.Result {
	resp, ok := replay.GetKeyFrame(region, id, frame)
	if !ok {
		return c.NotFound("")
	}

	decryptAndPrint(resp)

	return OctetResponse(resp)
}

func (c App) Catch(all string) revel.Result {
	revel.WARN.Println("404: " + all)
	return c.NotFound("")
}
