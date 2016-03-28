package main

import (
	"compress/gzip"
	"github.com/dustin/go-humanize"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type playerArg struct {
	ChampionImage string
	ChampionName  string
	Summoner      string
}

type recordingArg struct {
	IsComplete       bool
	NoMetadata       bool
	Players          []playerArg
	Recording        bool
	Ago              string
	Queue            string
	CapitalizedQueue string
	Region           string
	AQueue           string
	Code             string
	LegendsGG        string
}

type renderArg struct {
	Recordings   []recordingArg
	CurrentPage  int
	Pages        []int
	NextPage     int
	PreviousPage int
	LoadTime     string
}

var pageTemplate *template.Template

func serveView(w http.ResponseWriter, r *http.Request) {
	var currentPage int
	if r.URL.Path == "/" {
		currentPage = 1
	} else {
		num, err := strconv.Atoi(r.URL.Path[1:])
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 page not found"))
			return
		}

		currentPage = num
	}

	gzippable := false
	if encodings, found := r.Header["Accept-Encoding"]; found {
		for _, enc := range encodings {
			if enc == "gzip" {
				gzippable = true
				break
			}
		}
	}

	templateRenderArg := getRenderArg(r, currentPage)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if gzippable {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)

		writer := gzip.NewWriter(w)
		if err := pageTemplate.Execute(writer, templateRenderArg); err != nil {
			writer.Write([]byte("template render error"))
			log.Println("render: template error:", err)
		}
		writer.Close()
	} else {
		w.WriteHeader(http.StatusOK)
		if err := pageTemplate.Execute(w, templateRenderArg); err != nil {
			w.Write([]byte("template render error"))
			log.Println("render: template error:", err)
		}
	}
}

func getRenderArg(r *http.Request, currentPage int) renderArg {
	start := time.Now()

	recordingsMutex.RLock()
	defer recordingsMutex.RUnlock()

	numPages := int(math.Ceil(float64(len(sortedRecordings)) /
		float64(config.ShowPerPage)))

	renderTemplateArg := renderArg{
		CurrentPage:  currentPage,
		Pages:        makePages(numPages),
		NextPage:     currentPage + 1,
		PreviousPage: currentPage - 1,
		Recordings:   make([]recordingArg, 0, config.ShowPerPage),
	}

	for i := len(sortedRecordings) - ((currentPage - 1) *
		config.ShowPerPage) - 1; i >= 0 && i > len(sortedRecordings)-
		(currentPage*config.ShowPerPage)-1; i-- {

		var recRenderArg recordingArg
		rec := sortedRecordings[i]

		if rec.temporary {
			continue
		}

		var game gameInfoMetadata
		err := rec.rec.RetrieveUserMetadata(&game)
		info := rec.rec.RetrieveGameInfo()

		recRenderArg.Recording = rec.recording
		recRenderArg.Region = strings.ToUpper(platformToRegion[info.Platform])

		if err != nil {
			if rec.recording {
				continue
			}

			log.Println("render: failed to get metadata for "+rec.location+":",
				err)
			recRenderArg.NoMetadata = true
			recRenderArg.Ago = capitalize(humanize.Time(info.RecordTime))
		} else {
			recRenderArg.NoMetadata = false

			if !rec.recording {
				recRenderArg.IsComplete = rec.rec.IsComplete()
			} else {
				recRenderArg.IsComplete = true
			}

			recRenderArg.Queue = getQueue(game.GameQueueConfigID)
			recRenderArg.AQueue = aOrAn(recRenderArg.Queue)
			recRenderArg.CapitalizedQueue = capitalize(recRenderArg.Queue)
			recRenderArg.Ago = humanize.Time(info.RecordTime)
		}

		host := strings.Split(r.Host, ":")

		codeBody := host[0] + ":" + strconv.Itoa(config.ShowReplayPortAs) +
			" " + info.EncryptionKey + " " + info.GameId + " " + info.Platform

		if !recRenderArg.IsComplete {
			recRenderArg.Code = "spectator " + codeBody
		} else {
			recRenderArg.Code = "replay " + codeBody
		}

		if staticDataAvailable {
			// Find people in the game
			championsMutex.RLock()
			for _, player := range game.Participants {
				var thisPlayer playerArg

				// Find player in monitor list
				match := false
				for _, monitoredPlayer := range config.Players {
					if monitoredPlayer.Platform != info.Platform {
						continue
					}

					if monitoredPlayer.ID != strconv.FormatInt(player.SummonerID, 10) {
						continue
					}

					match = true
					break
				}

				if !match {
					continue
				}

				thisPlayer.Summoner = player.SummonerName

				if champion, found := allChampions[player.ChampionID]; found {
					thisPlayer.ChampionName = champion.Name
					thisPlayer.ChampionImage =
						"https://ddragon.leagueoflegends.com/cdn/" +
							currentVersion + "/img/champion/" + champion.Image.Full
				} else {
					thisPlayer.ChampionName = "an unknown champion"
					thisPlayer.ChampionImage = ""
					log.Println("render: missing champion:", player.ChampionID)
				}

				recRenderArg.Players = append(recRenderArg.Players, thisPlayer)
			}
			championsMutex.RUnlock()
		}

		if rec.recording {
			if len(recRenderArg.Players) > 0 {
				recRenderArg.LegendsGG = "http://www.legends.gg/current/" +
					platformToRegion[info.Platform] + "/" +
					recRenderArg.Players[0].Summoner
			} else if len(game.Participants) > 0 {
				recRenderArg.LegendsGG = "http://www.legends.gg/current/" +
					platformToRegion[info.Platform] + "/" +
					game.Participants[0].SummonerName
			} else {
				recRenderArg.LegendsGG = ""
			}
		} else {
			if len(recRenderArg.Players) > 0 {
				recRenderArg.LegendsGG = "http://www.legends.gg/" +
					platformToRegion[info.Platform] + "/" +
					recRenderArg.Players[0].Summoner + "/match/" + info.GameId
			} else if len(game.Participants) > 0 {
				recRenderArg.LegendsGG = "http://www.legends.gg/" +
					platformToRegion[info.Platform] + "/" +
					game.Participants[0].SummonerName + "/match/" + info.GameId
			} else {
				recRenderArg.LegendsGG = "http://www.legends.gg/" +
					platformToRegion[info.Platform] + "/_/match/" + info.GameId
			}
		}

		renderTemplateArg.Recordings =
			append(renderTemplateArg.Recordings, recRenderArg)
	}

	renderTemplateArg.LoadTime = strconv.FormatFloat(time.Now().Sub(start).Seconds()*1000.0,
		'f', 1, 64)

	return renderTemplateArg
}

func makePages(numPages int) []int {
	result := make([]int, numPages)
	for i := 1; i <= numPages; i++ {
		result[i-1] = i
	}
	return result
}

func capitalize(str string) string {
	return strings.ToUpper(str[:1]) + str[1:]
}

func aOrAn(word string) string {
	switch word[0] {
	case 'a', 'A', 'e', 'E', 'i', 'I', 'o', 'O', 'u', 'U':
		return "An"
	default:
		return "A"
	}
}

func getQueue(id int) string {
	switch id {
	case 0:
		return "custom"
	case 8:
		return "twisted treeline"
	case 2:
		return "normal blind"
	case 14:
		return "normal draft"
	case 4:
		return "ranked"
	case 6:
		return "twisted treeline ranked"
	case 9:
		return "ranked"
	case 41:
		return "twisted treeline ranked team"
	case 42:
		return "ranked team"
	case 61:
		return "normal team builder"
	case 65:
		return "ARAM"
	case 70:
		return "one for all"
	case 72:
		return "snowdown showdown"
	case 73:
		return "snowdown showdown"
	case 75:
		return "hexakill"
	case 76:
		return "ultra rapid fire"
	case 83:
		return "ultra rapid fire"
	case 91:
		return "doom bots one"
	case 92:
		return "doom bots two"
	case 93:
		return "doom bots five"
	case 96:
		return "ascension"
	case 98:
		return "hexakill"
	case 100:
		return "bilgewater ARAM"
	case 300:
		return "Legend of the Poro King"
	case 310:
		return "counter pick"
	case 313:
		return "normal blind"
	case 400:
		return "dynamic queue unranked"
	case 410:
		return "dynamic queue ranked"
	default:
		return "unknown queue type"
	}
}

func init() {
	pageTemplate = template.Must(template.New("page").Parse(pageSource))
}

var pageSource = `<!DOCTYPE html>
<html>
<head>
	<title>LoL Replay</title>
	<link rel="stylesheet" type="text/css" href="/bulma.min.css">
	<link rel="stylesheet" type="text/css" href="/style.css">
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.5.0/css/font-awesome.min.css">
</head>
<body>
<section class="hero">
	<div class="hero-content">
		<div class="container">
			<h1 class="title">
				LoL Replay
			</h1>
			<h2 class="subtitle">
				Self hosted League of Legends replays
			</h2>
		</div>
	</div>
</section>
<section class="section">
	<div class="container">
		<div class="masonry">
			{{- range $recording := .Recordings}}
			<div class="masonry-half">
				<div class="card is-fullwidth">
					{{- if .NoMetadata}}
					<header class="card-header no-metadata">
						<p class="card-header-title">
						Metadata unavailable (possibly corrupt)
						</p>
					</header>
					{{- else}}
					{{- if not .IsComplete}}
					<header class="card-header corrupt">
						<p class="card-header-title">
						Possibly incomplete recording
						</p>
					</header>
					{{- end}}
					{{- end}}
					<div class="card-content">
						{{- if not .NoMetadata}}
						<div class="columns is-multiline">
							{{- range $player := .Players}}
							<div class="column is-half">
								<div class="media">
									<div class="media-left">
										<figure class="image is-48x48">
											<img src="{{.ChampionImage}}" alt="{{.ChampionName}}">
										</figure>
									</div>
									<div class="media-content">
										<p class="title is-5">{{.Summoner}}</p>
										{{- if $recording.Recording}}
										<p class="subtitle is-6">is playing {{.ChampionName}}</p>
										{{- else}}
										<p class="subtitle is-6">played {{.ChampionName}}</p>
										{{- end}}
									</div>
								</div>
							</div>
							{{- end}}
						</div>
						{{- if .Recording}}
						<p><span class="record"></span>{{.CapitalizedQueue}} game being recorded on {{.Region}}...</p>
						{{- else}}
						<p>{{.CapitalizedQueue}} game played {{.Ago}} on {{.Region}}.</p>
						{{- end}}
						{{- else}}
						<p>{{.Ago}} on {{.Region}}.</p>
						{{- end}}
						{{- if not .Recording}}
						<div class="code-area">
							<textarea readonly>{{.Code}}</textarea>
						</div>
						{{- end}}
					</div>
					<footer class="card-footer">
						{{- if not .Recording}}
						<a class="card-footer-item" onclick="copyCode(this)">Copy to clipboard</a>
						{{- end}}
						<a class="card-footer-item" href="{{.LegendsGG}}" target="_blank">Show on Legends.GG<i class="fa fa-external-link"></i></a>
					</footer>
				</div>
			</div>
			{{- end}}
		</div>
		<div style="clear:both;"></div>
		<nav class="pagination">
			{{- if eq .CurrentPage 1}}
			<a disabled>Previous page</a>
			{{- else}}
			<a href="/{{.PreviousPage}}">Previous page</a>
			{{- end}}
			{{- if eq .CurrentPage (len .Pages)}}
			<a disabled>Next page</a>
			{{- else}}
			<a href="/{{.NextPage}}">Next page</a>
			{{- end}}
			<ul>
				{{- range $element := .Pages}}
					<li>
						{{- if eq . $.CurrentPage}}
						<a class="is-active">{{.}}</a>
						{{- else}}
						<a href="/{{.}}">{{.}}</a>
						{{- end}}
					</li>
				{{- end}}
			</ul>
		</nav>
	</div>
</section>
<footer class="footer">
	<div class="container">
		<div class="content">
			<p>
			Page took {{.LoadTime}} ms to render.
			</p>
			<p>
			<strong>LoL Replay</strong> by <a href="https://github.com/1lann" target="_blank">1lann</a>. The <a href="https://github.com/1lann/lol-replay" target="_blank">source code</a> is licensed under the <a href="https://github.com/1lann/lol-replay/blob/master/LICENSE" target="_blank">MIT license</a>.
			</p>
			<p>
			LoL Replay isn't endorsed by Riot Games and doesn't reflect the views or opinions of Riot Games or anyone officially involved in producing or managing League of Legends. League of Legends and Riot Games are trademarks or registered trademarks of Riot Games, Inc.
			</p>
		</div>
	</div>
</footer>
<script type="text/javascript">
var masonry = document.getElementsByClassName("masonry");

var leftColumnHeight = 0
var rightColumnHeight = 0;

for (var i = 0; i < masonry[0].children.length; i++) {
	var elem = masonry[0].children[i];
	if (leftColumnHeight > rightColumnHeight) {
		elem.className += " masonry-right";
		rightColumnHeight += elem.clientHeight;
	} else {
		leftColumnHeight += elem.clientHeight;
	}
}

var copyCode = function(elem) {
	if (elem.innerText != "Copy to clipboard") {
		return;
	}

	var card = elem.parentElement.parentElement;
	var cardContent;
	for (var i = 0; i < card.children.length; i++) {
		if (card.children[i].className == "card-content") {
			cardContent = card.children[i];
			break;
		}
	}

	var textarea;
	for (var i = 0; i < cardContent.children.length; i++) {
		if (cardContent.children[i].className == "code-area") {
			textarea = cardContent.children[i].children[0];
			break;
		}
	}

	textarea.select();

	try {
		if (document.execCommand("copy")) {
			flashButton(elem, true);
		} else {
			flashButton(elem, false);
		}
	} catch (err) {
		flashButton(elem, false);
	}
}

var flashButton = function(elem, success) {
	if (success) {
		window.getSelection().removeAllRanges()
		elem.style.color = "#4CAF50";
		elem.innerText = "Copied!";

		setTimeout(function() {
			elem.style.color = "";
			elem.innerText = "Copy to clipboard";
		}, 2000);
	} else {
		elem.style.color = "#009688";

		if (navigator.userAgent.indexOf("Safari") > -1) {
			elem.innerText = "Press ⌘+C to copy";
		} else {
			elem.innerText = "Press Ctrl+C to copy";
		}

		setTimeout(function() {
		elem.style.color = "";
		elem.innerText = "Copy to clipboard";
	}, 4000);
	}
}
</script>
</body>
</html>`
