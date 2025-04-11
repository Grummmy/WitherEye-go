package main

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/rivo/tview"
	"github.com/tidwall/gjson"
)

type Payload struct {
	Cheats      []string `json:"cheats"`
	MCLaunchers []string `json:"minecraft-launchers"`
	All         []string `json:"all"`
}

func loadData() (data Payload) {
	resp, err := http.Get("https://raw.githubusercontent.com/Grummmy/WitherEye/refs/heads/main/data.json")
	if err != nil {
		log.Fatal("Getting data error: ", err)
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		log.Fatal("JSON-parsing data error: ", err)
	}

	return data
}

func getRegex(data Payload) (cheats, islibptn, mcptn *regexp.Regexp) {
	cheats = regexp.MustCompile(`(?i)\b(` + strings.Join(data.Cheats, "|") + `)\b`)
	islibptn = regexp.MustCompile(`(?i)\b(` + strings.Join(data.MCLaunchers, "|") + `)\b.*\b(lib|libs|library|libraries)\b`)
	mcptn = regexp.MustCompile(`(?i)\b(` + strings.Join(data.MCLaunchers, "|") + `|downloads)\b`)

	return cheats, islibptn, mcptn
}

func getSha512(path string) string {
	f, err := os.Open(path)
    if err != nil {
        log.Fatal("Opening file erro: ", err)
    }
	defer f.Close()

	h := sha512.New()
    if _, err := io.Copy(h, f); err != nil {
        log.Fatal("Creating hash error: ", err)
    }

	return fmt.Sprintf("%x", h.Sum(nil))
}

func checkmod(sha512 string) (filename string, found bool) {
	resp, err := http.Get(fmt.Sprintf("https://api.modrinth.com/v2/version_file/%s", sha512))
	if err != nil {
		log.Fatal("Getting mod error: ", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", false
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading modrinth response: ", err)
	}
	
	filename = gjson.Get((string(bodyBytes)), "files.0.filename").String()
	return filename, true
}

func getFiles(search string, port int) (filenum int64, results []gjson.Result) {
	if port == -1 {
		port = 80
	}

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d?s=%s&j=1&path_column=1",port, url.QueryEscape(search)))
	if err != nil {
		log.Fatal("Getting files error: ", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading everything response: ", err)
	}

	return gjson.Get(string(bodyBytes), "totalResults").Int(), gjson.Get(string(bodyBytes), "results").Array()
}

func checkConn() int {
	resp, err := http.Get("http://localhost:80")
	defer fmt.Print("\033[H\033[2J")
	if err != nil {
		fmt.Println("Please, make sure that Everything http-server is running.")
		fmt.Print("If port is not 80, please, enter it or press enter to try again: ")
		
		var port int
		_, err := fmt.Scanf("%d", &port)
		if err != nil {
			fmt.Println("Invalid input.")
			port = checkConn()
		}

		return port
	}
	defer resp.Body.Close()
	return -1
}

func scan(app *tview.Application, list *tview.List, port int) {
	data := loadData()
	cheats, islibptn, mcptn := getRegex(data)

	l, files := getFiles(fmt.Sprintf(
		"ext:jar <!path:.fabric !path:.forge path:mod path:%s>|<path:download|path:$Recycle.Bin>",
		strings.Join(data.MCLaunchers, "|path:"),
	), port)

	for i, file := range files {
		name := file.Get("name").String()
		path := file.Get("path").String()
		pn := path + string(os.PathSeparator) + name

		found := true
		if file.Get("type").String() == "file" && mcptn.MatchString(path) && !islibptn.MatchString(path) {
			var n string
			n, found = checkmod(getSha512(pn))
			if found {
				name = n
				pn = path + string(os.PathSeparator) + n
			}
		}

		if match := cheats.FindString(name); match != "" {
			result := cheats.ReplaceAllStringFunc(pn, func(match string) string {
				return "[::r]" + match + "[::R]"
			})
			color := ""
			if !found {color = "[yellow]"}

			app.QueueUpdateDraw(func() {
				list.AddItem(color+result, " ┗╸ "+path, 0, func() {
					exec.Command("explorer", "/select,", pn).Start()
				})
			})
		}
		app.QueueUpdateDraw(func() {
			list.SetTitle(fmt.Sprintf(" %d / %d ", i+1, l))
		})
	}	
}

func main() {
	port := checkConn()

	list := tview.NewList()
	app := tview.NewApplication().SetRoot(list, true)
	list.SetBorder(true).SetTitle(" loading... ").SetTitleAlign(tview.AlignCenter)

	go scan(app, list, port)

	if err := app.Run(); err != nil {
		panic(err)
	}
}
