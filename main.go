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
	"regexp"
	"strings"

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
	// search = fmt.Sprintf("http://localhost:%d?s=%s&j=1&path_column=1",port, url.QueryEscape(search))
	// fmt.Println(search)
	// resp, err := http.Get(search)
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

func main() {
	data := loadData()
	cheats, islibptn, mcptn := getRegex(data)

	_, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	l, files := getFiles(fmt.Sprintf(
		"ext:jar <!path:.fabric !path:.forge path:mod path:%s>|<path:download|path:$Recycle.Bin>",
		strings.Join(data.MCLaunchers, "|path:"),
	), -1)
	fmt.Println("got files: ", l)
	for _, file := range files {
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
				return "\033[1m" + match + "\033[22m"
			})
			if !found {
				fmt.Printf("\033[33mSuspisious: %s\033[0m\n", result)
			} else {
				fmt.Println(result)
			}
			fmt.Println(" ┗╸ ", path)
		}
	}


	var enter string
	fmt.Print("Press enter to exit... ")
	fmt.Scanln(&enter)
}
