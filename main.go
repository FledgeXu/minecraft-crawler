package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

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

	for letter := range check {
		res = append(res, letter)
	}

	return res
}

func DownloadFile(jobs <-chan [2]string, wg *sync.WaitGroup) error {
	defer wg.Done()
	for job := range jobs {
		// Get the data
		fmt.Println(job)
		resp, err := http.Get(job[0])
		if err != nil {
			fmt.Println(err)
			return err
		}

		// Create the file
		if _, err := os.Stat(filepath.Dir(job[1])); os.IsNotExist(err) {
			err := os.MkdirAll(filepath.Dir(job[1]), os.ModePerm)
			if err != nil {
				fmt.Println(err)
				return err
			}
		}
		out, err := os.Create(job[1])
		if err != nil {
			fmt.Println(err)
			return err
		}

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			fmt.Println(err)
			return err
		}
		resp.Body.Close()
		out.Close()
	}
	return nil
}

func versionWorker(jobs <-chan string, result chan<- VersionWorkerResult) {
	for j := range jobs {
		resp, err := http.Get(j)
		if err != nil {
			fmt.Println(err)
			panic("Can't get version Info.")
		}
		body, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
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
		defer resp.Body.Close()
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
		if worker_result.client != "" {
			client_jars = append(client_jars, worker_result.client)
		}
		if worker_result.client_mappings != "" {
			client_mappings = append(client_mappings, worker_result.client_mappings)
		}
		if worker_result.server != "" {
			server_jars = append(server_jars, worker_result.server)
		}
		if worker_result.server_mappings != "" {
			server_mappings = append(server_mappings, worker_result.server_mappings)
		}
		libraries = appendCategory(libraries, worker_result.libraies)
		objects = appendCategory(objects, worker_result.asserts)
	}

	var wg sync.WaitGroup
	donwload_jobs := make(chan [2]string, len(libraries))
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go DownloadFile(donwload_jobs, &wg)
	}
	for _, library_url := range libraries {
		urlObject, err := url.Parse(library_url)
		if err != nil {
			fmt.Println(err)
			panic("parse url fails.")
		}
		var a [2]string
		a[0] = library_url
		a[1] = fmt.Sprintf("./cache/library%s", urlObject.Path)
		donwload_jobs <- a
		fmt.Println(urlObject.Path)
	}
	close(donwload_jobs)
	wg.Wait()

	donwload_jobs = make(chan [2]string, len(objects))
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go DownloadFile(donwload_jobs, &wg)
	}
	for _, object := range objects {
		var a [2]string
		a[0] = fmt.Sprintf("https://resources.download.minecraft.net/%s/%s", object[:2], object)
		a[1] = fmt.Sprintf("./cache/asserts/objects/%s/%s", object[:2], object)
		donwload_jobs <- a
		fmt.Println(a[1])
	}
	close(donwload_jobs)
	wg.Wait()

	donwload_jobs = make(chan [2]string, len(client_jars))
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go DownloadFile(donwload_jobs, &wg)
	}
	for _, client_jar_url := range client_jars {
		urlObject, err := url.Parse(client_jar_url)
		if err != nil {
			fmt.Println(err)
			panic("parse url fails.")
		}
		var a [2]string
		a[0] = client_jar_url
		a[1] = fmt.Sprintf("./cache/jars%s", urlObject.Path)
		donwload_jobs <- a
		fmt.Println(urlObject.Path)
	}
	close(donwload_jobs)
	wg.Wait()

	donwload_jobs = make(chan [2]string, len(client_mappings))
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go DownloadFile(donwload_jobs, &wg)
	}
	for _, client_mapping_url := range client_mappings {
		urlObject, err := url.Parse(client_mapping_url)
		if err != nil {
			fmt.Println(err)
			panic("parse url fails.")
		}
		var a [2]string
		a[0] = client_mapping_url
		a[1] = fmt.Sprintf("./cache/jars%s", urlObject.Path)
		donwload_jobs <- a
		fmt.Println(urlObject.Path)
	}
	close(donwload_jobs)
	wg.Wait()

	donwload_jobs = make(chan [2]string, len(server_jars))
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go DownloadFile(donwload_jobs, &wg)
	}
	for _, server_jar_url := range server_jars {
		urlObject, err := url.Parse(server_jar_url)
		if err != nil {
			fmt.Println(err)
			panic("parse url fails.")
		}
		var a [2]string
		a[0] = server_jar_url
		a[1] = fmt.Sprintf("./cache/jars%s", urlObject.Path)
		donwload_jobs <- a
		fmt.Println(urlObject.Path)
	}
	close(donwload_jobs)
	wg.Wait()

	donwload_jobs = make(chan [2]string, len(server_mappings))
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go DownloadFile(donwload_jobs, &wg)
	}
	for _, server_mapping_url := range server_mappings {
		urlObject, err := url.Parse(server_mapping_url)
		if err != nil {
			fmt.Println(err)
			panic("parse url fails.")
		}
		var a [2]string
		a[0] = server_mapping_url
		a[1] = fmt.Sprintf("./cache/jars%s", urlObject.Path)
		donwload_jobs <- a
		fmt.Println(urlObject.Path)
	}
	close(donwload_jobs)
	wg.Wait()
}
