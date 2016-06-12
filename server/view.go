package main

import (
	"compress/gzip"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/1lann/lol-replay/recording"
	"github.com/dustin/go-humanize"
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
	Duration         string
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

	recordingsMutex.RLock()
	numRecordings := len(sortedRecordings)
	recordingsMutex.RUnlock()

	if (currentPage > int(math.Ceil(float64(numRecordings)/
		float64(config.ShowPerPage))) && currentPage != 1) ||
		currentPage < 1 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	templateRenderArg := getRenderArg(r, currentPage)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
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

func getLegendsGGLink(recording bool, recRenderArg recordingArg,
	info recording.GameInfo, game gameInfoMetadata) string {
	if recording {
		if len(recRenderArg.Players) > 0 {
			return "http://www.legends.gg/current/" +
				platformToRegion[info.Platform] + "/" +
				recRenderArg.Players[0].Summoner
		} else if len(game.Participants) > 0 {
			return "http://www.legends.gg/current/" +
				platformToRegion[info.Platform] + "/" +
				game.Participants[0].SummonerName
		} else {
			return ""
		}
	} else {
		if len(recRenderArg.Players) > 0 {
			return "http://www.legends.gg/" +
				platformToRegion[info.Platform] + "/" +
				recRenderArg.Players[0].Summoner + "/match/" + info.GameID
		} else if len(game.Participants) > 0 {
			return "http://www.legends.gg/" +
				platformToRegion[info.Platform] + "/" +
				game.Participants[0].SummonerName + "/match/" + info.GameID
		} else {
			return "http://www.legends.gg/" +
				platformToRegion[info.Platform] + "/_/match/" + info.GameID
		}
	}
}

func getPlayerArg(summonerID int64, summonerName string, championID int,
	info recording.GameInfo) (playerArg, bool) {
	var thisPlayer playerArg

	// Find player in monitor list
	match := false
	for _, monitoredPlayer := range config.Players {
		if monitoredPlayer.Platform != info.Platform {
			continue
		}

		if monitoredPlayer.ID != strconv.FormatInt(summonerID, 10) {
			continue
		}

		match = true
		break
	}

	if !match {
		return playerArg{}, false
	}

	thisPlayer.Summoner = summonerName

	if champion, found := allChampions[championID]; found {
		thisPlayer.ChampionName = champion.Name
		thisPlayer.ChampionImage =
			"https://ddragon.leagueoflegends.com/cdn/" +
				currentVersion + "/img/champion/" + champion.Image.Full
	} else {
		thisPlayer.ChampionName = "an unknown champion"
		thisPlayer.ChampionImage = ""
		log.Println("render: missing champion:", championID)
	}

	return thisPlayer, true
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

		duration := int(rec.rec.LastWriteTime().Sub(info.RecordTime).Minutes())

		if err != nil {
			if rec.recording {
				continue
			}

			log.Println("render: failed to get metadata for "+rec.location+":",
				err)
			recRenderArg.NoMetadata = true
			recRenderArg.Ago = capitalize(humanize.Time(
				rec.rec.LastWriteTime()))
			recRenderArg.Duration = strconv.Itoa(duration) + " minutes"
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
			recRenderArg.Ago = humanize.Time(rec.rec.LastWriteTime())
			recRenderArg.Duration = strconv.Itoa(duration) + " minutes"
		}

		host := strings.Split(r.Host, ":")

		codeBody := host[0] + ":" + strconv.Itoa(config.ShowReplayPortAs) +
			" " + info.EncryptionKey + " " + info.GameID + " " + info.Platform

		recRenderArg.Code = "replay " + codeBody

		staticDataMutex.Lock()
		if staticDataAvailable {
			staticDataMutex.Unlock()
			// Find people in the game
			championsMutex.RLock()
			for _, player := range game.Participants {
				playerArgs, ok := getPlayerArg(player.SummonerID,
					player.SummonerName, player.ChampionID, info)
				if !ok {
					continue
				}
				recRenderArg.Players = append(recRenderArg.Players, playerArgs)
			}
			championsMutex.RUnlock()
		} else {
			staticDataMutex.Unlock()
		}

		recRenderArg.LegendsGG = getLegendsGGLink(rec.recording,
			recRenderArg, info, game)

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

var allQueues = map[int]string{
	0:   "custom",
	8:   "twisted treeline",
	2:   "normal blind",
	14:  "normal draft",
	4:   "ranked",
	6:   "twisted treeline ranked",
	9:   "ranked",
	41:  "twisted treeline ranked team",
	42:  "ranked team",
	61:  "normal team builder",
	65:  "ARAM",
	70:  "one for all",
	72:  "snowdown showdown",
	73:  "snowdown showdown",
	75:  "hexakill",
	76:  "ultra rapid fire",
	83:  "ultra rapid fire",
	91:  "doom bots one",
	92:  "doom bots two",
	93:  "doom bots five",
	96:  "ascension",
	98:  "hexakill",
	100: "bilgewater ARAM",
	300: "Legend of the Poro King",
	310: "counter pick",
	313: "normal blind",
	400: "dynamic queue unranked",
	410: "dynamic queue ranked",
}

func getQueue(id int) string {
	queue, found := allQueues[id]
	if !found {
		return "unknown queue type"
	}

	return queue
}

func init() {
	pageTemplate = template.Must(template.New("page").Parse(pageSource))
}

var pageSource = `<!DOCTYPE html>
<html>
<head>
	<title>LoL Replay</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link rel="stylesheet" type="text/css" href="/bulma.min.css">
	<link rel="stylesheet" type="text/css" href="/style.css">
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.5.0/css/font-awesome.min.css">
</head>
<body>
<section class="hero">
	<div class="hero-content has-text-centered">
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
						Incomplete recording
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
						<p><span class="record"></span>{{.CapitalizedQueue}} game being recorded on {{.Region}} for {{.Duration}}...</p>
						{{- else}}
						<p>A {{.Duration}} {{.Queue}} game played {{.Ago}} on {{.Region}}.</p>
						{{- end}}
						{{- else}}
						<p>A {{.Duration}} game recorded {{.Ago}} on {{.Region}}.</p>
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
			<a disabled>Previous</a>
			{{- else}}
			{{- if eq .PreviousPage 1}}
			<a href="/">Previous</a>
			{{- else}}
			<a href="/{{.PreviousPage}}">Previous</a>
			{{- end}}
			{{- end}}
			{{- if eq .CurrentPage (len .Pages)}}
			<a disabled>Next</a>
			{{- else}}
			<a href="/{{.NextPage}}">Next</a>
			{{- end}}
			<ul>
				{{- range $element := .Pages}}
					<li>
						{{- if eq . $.CurrentPage}}
						<a class="is-active">{{.}}</a>
						{{- else}}
						{{- if eq . 1}}
						<a href="/">{{.}}</a>
						{{- else}}
						<a href="/{{.}}">{{.}}</a>
						{{- end}}
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
			<strong>LoL Replay</strong> by <a href="https://github.com/1lann" target="_blank">1lann</a>. The <a href="https://github.com/1lann/lol-replay" target="_blank">source code</a> is licensed under the <a href="https://github.com/1lann/lol-replay/blob/master/LICENSE" target="_blank">MIT license</a>.
			</p>
			<p>
			Page took {{.LoadTime}} ms to render.
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
		try {
			window.getSelection().removeAllRanges();
		} catch (err) {}
		try {
			document.selection.empty();
		} catch (err) {}

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
