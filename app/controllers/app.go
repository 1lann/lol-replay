package controllers

import (
	"github.com/revel/revel"
	"net/http"
	"replay/app/models/history"
	"replay/app/models/record"
	"replay/app/models/replay"
	"time"
)

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

	return OctetResponse(resp)
}

func (c App) KeyFrame(region, id, frame string) revel.Result {
	resp, ok := replay.GetKeyFrame(region, id, frame)
	if !ok {
		return c.NotFound("")
	}

	return OctetResponse(resp)
}

func (c App) Catch(all string) revel.Result {
	revel.WARN.Println("404: " + all)
	return c.NotFound("")
}
