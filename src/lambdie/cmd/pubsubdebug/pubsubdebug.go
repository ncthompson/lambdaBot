package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin/json"
	"github.com/pubsubhubbub/gohubbub"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	host := flag.String("host", "", "Host or IP to serve from")
	port := flag.Int("port", 10000, "The port to serve from")
	youtubeId := flag.String("youtubeId", "", "Set the youtube ID of the channel to follow")
	slackHook := flag.String("slackHook", "", "Set the Slack hook for the correct slack server")

	flag.Parse()

	log.Println("Youtube channel watcher started...")
	sub := "https://www.youtube.com/xml/feeds/videos.xml?channel_id=" + *youtubeId
	client := gohubbub.NewClient("https://pubsubhubbub.appspot.com", *host, *port, "Youtube Slacker")
	client.Subscribe(sub,
		func(contentType string, body []byte) {
			var feed Feed
			xmlError := xml.Unmarshal(body, &feed)

			if xmlError != nil {
				log.Printf("XML Parse Error %v", xmlError)

			} else {
				for _, entry := range feed.Entries {
					log.Printf("%s by %s (%s)", entry.Title, entry.Author.Name, entry.URL)
					postEntryToSlack(entry, *slackHook)
				}
			}
		})

	go client.StartServer()

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGTERM, syscall.SIGINT)
	<-termChan
	go func() {
		time.Sleep(10 * time.Second)
		panic("Unclean shutdown.")
	}()

	client.Unsubscribe(sub)

	time.Sleep(time.Second * 5)
}

type setup struct {
	host      string
	port      int
	youtubeId string
	slackUrl  string
}

type Feed struct {
	Status  string  `xml:"status>http"`
	Entries []Entry `xml:"entry"`
}

type Entry struct {
	URL     string `xml:"id"`
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Author  Author `xml:"author"`
}

type Author struct {
	Name string `xml:"name"`
}

type slackPost struct {
	Text string `json:"text"`
}

func postEntryToSlack(entry Entry, url string) {
	httpCli := &http.Client{}
	content := "application/json"

	// Expecting format (yt:video:[id])
	split := strings.Split(entry.URL, ":")
	if len(split) < 3 {
		fmt.Printf("URL could not be decoded: %v\n", entry.URL)
	}

	slackMsg := fmt.Sprintf("%v <https://youtu.be/%v>", entry.Title, split[2])
	slackStruct := slackPost{slackMsg}
	slackByte, err := json.Marshal(slackStruct)

	dataRead := strings.NewReader(string(slackByte))
	resp, err := httpCli.Post(url, content, dataRead)
	if err != nil {
		fmt.Printf("Could not create post: %v\n", err)
	}
	fmt.Printf("Reponse: %v\n", resp.Status)
	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		fmt.Printf("Could not read reponse body %v\n", err)
	}
	fmt.Printf("Body: %v\n", buf.String())
}
