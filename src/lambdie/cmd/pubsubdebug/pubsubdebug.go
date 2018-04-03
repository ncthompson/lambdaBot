package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/pubsubhubbub/gohubbub"
	"log"
	"time"
)

var host = flag.String("host", "", "Host or IP to serve from")
var port = flag.Int("port", 10000, "The port to serve from")

func main() {
	raw := flag.Bool("raw", true, "Print raw http results.")
	flag.Parse()
	if *raw {
		r := gin.Default()
		r.Any("/", handleGin)
		r.Run(":8081")
	} else {
		medium()
	}
}

func handleGin(c *gin.Context) {
	fmt.Printf("Content type: %v\n", c.ContentType())
	data, err := c.GetRawData()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Page: %v\n", string(data))
	}
	c.Status(204)
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

func medium() {

	log.Println("Medium Story Watcher Started...")

	client := gohubbub.NewClient("http://medium.superfeedr.com", *host, *port, "Test App")
	client.Subscribe("https://medium.com/feed/latest", func(contentType string, body []byte) {
		var feed Feed
		xmlError := xml.Unmarshal(body, &feed)

		if xmlError != nil {
			log.Printf("XML Parse Error %v", xmlError)

		} else {
			for _, entry := range feed.Entries {
				log.Printf("%s by %s (%s)", entry.Title, entry.Author.Name, entry.URL)
			}
		}
	})

	go client.StartServer()

	time.Sleep(time.Second * 5)
	log.Println("Press Enter for graceful shutdown...")

	var input string
	fmt.Scanln(&input)

	client.Unsubscribe("https://medium.com/feed/latest")

	time.Sleep(time.Second * 5)
}
