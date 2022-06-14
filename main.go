package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

type Task struct {
	Url   string
	Title string
}

type Result struct {
	Url         string
	Title       string
	Description string
}

func main() {
	var (
		url         = flag.String("url", "https://hackernoon.com/", "Scrape hackernoon")
		title       = flag.String("title", `main[class^="Page__Content"] h2 a`, "Selector")
		description = flag.String("description", `div.tldr`, "TLDR")
		concurrency = flag.Int("concurrency", 3, "Concurrency")
	)

	tasks := make(chan Task)
	go generateTasks(*url, *title, tasks)

	results := make(chan Result)
	wg := new(sync.WaitGroup)
	wg.Add(*concurrency)
	go closeResults(wg, results)

	for i := 0; i < *concurrency; i++ {
		go pushToResult(wg, tasks, results, *description)
	}

	for r := range results {
		log.Printf("Url: %s\nTitle: %s\nDescription: %s\n\n\n", r.Url, r.Title, r.Description)
	}
}

func closeResults(wg *sync.WaitGroup, results chan Result) {
	wg.Wait()
	close(results)
}

func pushToResult(wg *sync.WaitGroup, tasks chan Task, results chan Result, description string) {
	defer wg.Done()

	for t := range tasks {
		selections, err := fetchRequest(t.Url, description)
		if err != nil {
			log.Fatal(err)
		}

		txt := cleanText(selections.Text())
		results <- Result{Url: t.Url, Title: t.Title, Description: txt}
	}
}

func generateTasks(url string, title string, tasks chan Task) {
	defer close(tasks)

	selections, err := fetchRequest(url, title)
	if err != nil {
		log.Fatal(err)
	}

	selections.Each(func(i int, s *goquery.Selection) {
		link, ok := s.Attr("href")
		if ok {
			txt := cleanText(s.Text())

			if !strings.Contains(link, "://") {
				url = strings.TrimSuffix(url, "/")
				link = strings.TrimPrefix(link, "/")
				link = fmt.Sprintf("%s/%s", url, link)
			}
			tasks <- Task{Url: link, Title: txt}
		}
	})
}

func fetchRequest(url string, title string) (*goquery.Selection, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("could not get request %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("too many requests")
		}

		return nil, fmt.Errorf("bad response from server: %v", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not parse response body: %v", err)
	}

	return doc.Find(title), nil
}

func cleanText(txt string) string {
	txt = strings.TrimSpace(txt)
	txt = strings.ReplaceAll(txt, "\n", " ")
	return txt
}
