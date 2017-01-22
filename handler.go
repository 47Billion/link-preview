package link_preview

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/47Billion/link-preview/oembed"
	"github.com/47Billion/link-preview/url2oembed"
	"github.com/jeffail/tunny"
)

const (
	DEFAULT_WAIT_TIMEOUT         = 7
	DEFAULT_WORKER_COUNT         = 10
	DEFAULT_HTML_BYTES_TO_READ   = 50000
	DEFAULT_BINARY_BYTES_TO_READ = 4096
)

func init() {
	fmt.Println("=>init handler")
}

type apiHandler struct {
	cache      *cache
	workerPool *tunny.WorkPool
}

func NewApiHandler(providersFile string, workerCount, maxHTMLBytesToRead, maxBinaryBytesToRead, waitTimeout int64,
	whiteListRanges, blackListRanges string, cache *cache) *apiHandler {
	buf, err := ioutil.ReadFile(providersFile)
	if err != nil {
		panic(err)
	}

	var whiteListNetworks []*net.IPNet
	if len(whiteListRanges) > 0 {
		if whiteListNetworks, err = stringsToNetworks(strings.Split(whiteListRanges, " ")); err != nil {
			panic(err)
		}
	}

	var blackListNetworks []*net.IPNet
	if len(blackListRanges) > 0 {
		if blackListNetworks, err = stringsToNetworks(strings.Split(blackListRanges, " ")); err != nil {
			panic(err)
		}
	}
	if 0 == maxHTMLBytesToRead {
		maxHTMLBytesToRead = DEFAULT_HTML_BYTES_TO_READ
	}

	if 0 == maxBinaryBytesToRead {
		maxBinaryBytesToRead = DEFAULT_BINARY_BYTES_TO_READ
	}

	if 0 == waitTimeout {
		waitTimeout = DEFAULT_WAIT_TIMEOUT
	}

	oe := oembed.NewOembed()
	oe.ParseProviders(bytes.NewReader(buf))

	workers := make([]tunny.TunnyWorker, workerCount)

	for i := range workers {
		p := url2oembed.NewParser(oe)
		p.MaxHTMLBodySize = maxHTMLBytesToRead
		p.MaxBinaryBodySize = maxBinaryBytesToRead
		p.WaitTimeout = time.Duration(waitTimeout) * time.Second
		p.BlacklistedIPNetworks = blackListNetworks
		p.WhitelistedIPNetworks = whiteListNetworks
		workers[i] = &(apiWorker{Parser: p})
	}

	pool, err := tunny.CreateCustomPool(workers).Open()
	if err != nil {
		panic(err)
	}

	return &apiHandler{cache: cache, workerPool: pool}
}

//Call this while your instance goes down to release workers
func (api *apiHandler) Release() {
	defer api.workerPool.Close()
	return
}

func (api *apiHandler) UrlInfo(inputUrl string) *oembed.Info {
	info, err := api.processUrl(inputUrl)
	if nil != err {
		log.Println("=>UrlInfo", inputUrl, err)
	}
	return info
}

func (api *apiHandler) HandleHttp(w http.ResponseWriter, r *http.Request) {
	//set default response headers
	w.Header().Set("Content-Type", "application/json")
	var input map[string]string
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&input)
	if err != nil {
		//handle error
		fmt.Println("Invalid json input", err)
		return
	}
	defer r.Body.Close()
	var inputUrl string
	var exists bool
	if inputUrl, exists = input["url"]; !exists {
		http.Error(w, "{\"status\": \"FAILED\", \"message\":\"Invalid URL\"}", 500)
		return
	}

	info, err := api.processUrl(inputUrl)
	if nil != err {
		log.Printf("Invalid URL provided: %s", inputUrl)
		http.Error(w, "{\"status\": \"FAILED\", \"message\":\"Invalid URL\"}", 500)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, info.String())
}

func (api *apiHandler) processUrl(inputUrl string) (*oembed.Info, error) {
	_, err := url.Parse(inputUrl)

	if err != nil {
		return nil, err
	}

	if nil != api.cache {
		if info := api.cache.getHandler(inputUrl); nil != info { //Item found in cache
			return info, nil
		}
	}

	//Not found in cache. Submitting to workerPool
	result, err := api.workerPool.SendWork(inputUrl)

	if err != nil {
		log.Printf("An unknown error occured: %s", err.Error())
		return nil, err
	}

	if data, ok := result.(*workerData); ok {
		if data.Status != 200 {
			return nil, errors.New("Unable to decode worker result")
		}
		info := data.Data.(*oembed.Info)
		if nil != api.cache {
			api.cache.setHandler(info, api.cache.ttl)
		}
		return info, nil
	}
	log.Print("Unable to decode worker result")
	return nil, errors.New("Unable to decode worker result")

}

// stringsToNetworks converts arrays of string representation of IP ranges into []*net.IPnet slice
func stringsToNetworks(ss []string) ([]*net.IPNet, error) {
	var result []*net.IPNet
	for _, s := range ss {
		_, network, err := net.ParseCIDR(s)
		if err != nil {
			return nil, err
		}
		result = append(result, network)
	}

	return result, nil
}
