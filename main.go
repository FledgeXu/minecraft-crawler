package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/tidwall/gjson"
)

type VersionWorkerResult struct {
	asserts  []string
	libraies []string
}

func versionWorker(jobs <-chan string, result chan<- VersionWorkerResult) {
	for j := range jobs {
		resp, err := http.Get(j)
		if err != nil {
			fmt.Println(err)
			panic("Can't get version Info.")
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
			panic("Can't read verison info body.")
		}
		var library_urls []string = make([]string, 0)
		for _, value := range gjson.Get(string(body), "libraries.#.downloads.artifact.url").Array() {
			library_urls = append(library_urls, value.String())
		}
		asserts_url := gjson.Get(string(body), "assetIndex.url").String()
		resp, err = http.Get(asserts_url)
		if err != nil {
			fmt.Println(err)
			panic("Can't get version Info.")
		}
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
			panic("Can't read verison info body.")
		}
		resultMap := gjson.Get(string(body), `objects`).Map()
		var object_urls []string = make([]string, 0)
		for _, element := range resultMap {
			object_urls = append(object_urls, element.Get("hash").String())
		}
		result <- VersionWorkerResult{library_urls, object_urls}
	}
}

func main() {
	resp, err := http.Get("http://launchermeta.mojang.com/mc/game/version_manifest.json")
	if err != nil {
		fmt.Println(err)
		panic("Can't get manifest.")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		panic("Can't read manifest body.")
	}
	version_urls := gjson.Get(string(body), "versions.#.url")

	jobs := make(chan string, 600)
	result := make(chan VersionWorkerResult, 600)
	for w := 0; w < 10; w++ {
		go versionWorker(jobs, result)
	}
	for _, value := range version_urls.Array() {
		jobs <- value.String()
	}
	close(jobs)
	for a := 0; a < len(version_urls.Array()); a++ {
		fmt.Println(<-result)
	}
}
