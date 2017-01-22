/**
* @author @47Billion
**/
package link_preview

import (
	"fmt"
	"log"
	"strings"

	"github.com/47Billion/link-preview/url2oembed"
)

type apiWorker struct {
	Parser *url2oembed.Parser
}

type workerData struct {
	Status int
	Data   interface{}
}

// Use this call to block further jobs if necessary
func (worker *apiWorker) TunnyReady() bool {
	return true
}

// This is where the work actually happens
func (worker *apiWorker) TunnyJob(data interface{}) interface{} {
	if u, ok := data.(string); ok {
		u = strings.Trim(u, "\r\n")

		info := worker.Parser.Parse(u)

		if info == nil {
			log.Printf("No info for url: %s", u)

			return &workerData{Status: 404, Data: "{\"status\": \"error\", \"message\":\"Unable to retrieve information from provided url\"}"}
		}
		if info.Status < 300 {
			log.Printf("Url parsed: %s", u)

			return &workerData{Status: 200, Data: info}
		}

		log.Printf("Something weird: %s", u)

		return &workerData{Status: 411, Data: fmt.Sprintf("{\"status\": \"error\", \"message\":\"Unable to obtain data. Status code: %d\"}", info.Status)}
	}

	return &workerData{Status: 500, Data: "{\"status\": \"error\", \"message\":\"Something weird happened\"}"}
}
