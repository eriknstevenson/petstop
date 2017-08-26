package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

const (
	defaultWorkerCount = 4
	defaultLimit       = 1000
	baseURL            = "http://marketplace.akc.org"
	searchEndpoint     = "/puppies"
)

func main() {

	logFile, err := os.OpenFile("log.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {

	}
	defer logFile.Close()
	log.SetOutput(logFile)

	workers := flag.Int("workers", defaultWorkerCount, "Number of threads to use to process jobs.")
	limit := flag.Int("limit", defaultLimit, "Amount of breeder records to fetch.")
	flag.Parse()

	csvSink(*workers, *limit)

}

func counter(start int) chan int {

	output := make(chan int, 16)

	go func() {
		defer close(output)
		for i := start; ; i++ {
			output <- i
		}
	}()

	return output
}

func searchResultStream(resultsPerPage int) chan string {

	output := make(chan string, 16)
	pageNumbers := counter(0)

	result, err := url.Parse(baseURL + searchEndpoint)
	if err != nil {
		log.Fatalf("unable to parse url %s: %v", baseURL, err)
	}

	go func() {
		defer close(output)
		for pageNumber := range pageNumbers {
			result.RawQuery = url.Values{
				"page":     {fmt.Sprintf("%d", pageNumber)},
				"per_page": {fmt.Sprintf("%d", resultsPerPage)},
			}.Encode()
			output <- result.String()
		}
	}()

	return output
}

func listingStream(workers int) chan string {

	output := make(chan string, 16)
	pageUrls := searchResultStream(40)

	var wg sync.WaitGroup
	wg.Add(workers)

	go func() {
		wg.Wait()
		close(output)
	}()

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for pageURL := range pageUrls {
				doc, err := goquery.NewDocument(pageURL)
				if err != nil {
					log.Printf("could not get page %s: %v", pageURL, err)
					continue
				}

				log.Printf("Downloaded page data from %s", pageURL)

				doc.Find(".litter-card").Each(func(i int, s *goquery.Selection) {
					listingEndpoint, exists := s.Find("a").Attr("href")
					if exists {
						output <- listingEndpoint
					}
				})
			}
		}()
	}

	return output
}

type BreederInfo struct {
	breed      string
	kennelName string
	name       string
	experience string
	location   string
	phone      string
	website    string
}

func breederInfoHeader() []string {
	return []string{
		"Breed", "Kennel Name", "Name", "Experience", "Location", "Phone", "Website",
	}
}

func (bi *BreederInfo) toRow() []string {
	return []string{
		bi.breed, bi.kennelName, bi.name, bi.experience, bi.location, bi.phone, bi.website,
	}
}

func breederStream(workers int) chan BreederInfo {

	output := make(chan BreederInfo, 16)
	listingEndpoints := listingStream(workers)

	var wg sync.WaitGroup
	wg.Add(workers)

	go func() {
		wg.Wait()
		close(output)
	}()

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()

			for listingEndpoint := range listingEndpoints {
				reqURL := baseURL + listingEndpoint
				doc, err := goquery.NewDocument(reqURL)
				if err != nil {
					log.Printf("could not get page %s: %v", reqURL, err)
					continue
				}

				var breeder BreederInfo

				doc.Find(".storefront__info").Find("p > strong").Each(func(i int, s *goquery.Selection) {

					key := s.Text()
					val := strings.TrimPrefix(s.Parent().Text(), key)
					key = strings.TrimSuffix(strings.TrimSpace(key), ":")
					val = strings.TrimSpace(val)

					switch key {
					case "Breed(s)":
						breeder.breed = val
					case "Kennel Name":
						breeder.kennelName = val
					case "Breeder Name":
						breeder.name = val
					case "Breeding for":
						breeder.experience = val
					case "Breeder's Location":
						breeder.location = val
					case "Contact By Phone":
						breeder.phone = val
					case "Website":
						breeder.website = val
					default:
						log.Printf("Encountered key %s that was not handled/stored.", key)
					}
				})

				output <- breeder

				log.Printf("Processed page %s", reqURL)

			}

		}()
	}
	return output
}

func csvSink(workers int, limit int) {
	breeders := breederStream(workers)
	count := 0

	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	w.Write(breederInfoHeader())

	for x := range breeders {

		w.Write(x.toRow())
		w.Flush()

		count++
		if count > limit {
			break
		}
	}
}
