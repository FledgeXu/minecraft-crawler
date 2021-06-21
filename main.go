package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/tidwall/gjson"
)

type VersionWorkerResult struct {
	client          string
	client_mappings string
	server          string
	server_mappings string
	asserts         []string
	libraies        []string
}

func appendCategory(a []string, b []string) []string {
	check := make(map[string]int)
	d := append(a, b...)
	res := make([]string, 0)
	for _, val := range d {
		check[val] = 1
	}

	for letter, _ := range check {
		res = append(res, letter)
	}

	return res
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

		client_url := gjson.Get(string(body), "downloads.client.url").String()
		client_mappings_url := gjson.Get(string(body), "downloads.client_mappings.url").String()
		server_url := gjson.Get(string(body), "downloads.server.url").String()
		server_mappings_url := gjson.Get(string(body), "downloads.server_mappings.url").String()

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
		result <- VersionWorkerResult{client_url, client_mappings_url, server_url, server_mappings_url, object_urls, library_urls}
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
	var client_jars []string = make([]string, 0)
	var client_mappings []string = make([]string, 0)
	var server_jars []string = make([]string, 0)
	var server_mappings []string = make([]string, 0)
	var libraries []string = make([]string, 0)
	var objects []string = make([]string, 0)
	for a := 0; a < len(version_urls.Array()); a++ {
		worker_result := <-result
		client_jars = append(client_jars, worker_result.client)
		client_mappings = append(client_mappings, worker_result.client_mappings)
		server_jars = append(server_jars, worker_result.server)
		server_mappings = append(server_mappings, worker_result.server_mappings)
		libraries = appendCategory(libraries, worker_result.libraies)
		objects = appendCategory(objects, worker_result.asserts)
	}
	fmt.Println(objects)
}
