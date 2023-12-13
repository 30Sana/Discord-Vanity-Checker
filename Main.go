package main

import (
	// "30SanaPkg/webhook"
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// var kernel32 = syscall.NewLazyDLL("kernel32.dll")
// var procSetConsoleTitleW = kernel32.NewProc("SetConsoleTitleW")

var isWindows = runtime.GOOS == "windows"

func setConsoleTitle(title string) {
	if isWindows {
		// titlePtr, _ := syscall.UTF16PtrFromString(title)
		// procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(titlePtr)))
	} else {
		fmt.Printf("\033]0;%s\007", title)
	}
}

func main() {
	proxies, err := loadProxies("data/proxies.txt")
	if err != nil {
		fmt.Println("Error loading proxies:", err)
		return
	}

	targets, err := loadTargets("data/targets.txt")
	if err != nil {
		fmt.Println("Error loading targets:", err)
		return
	}

	headers := map[string]string{
		"Origin":             "https://toolscord.com",
		"Sec-Ch-Ua":          "\"Chromium\";v=\"119\", \"Not?A_Brand\";v=\"24\"",
		"Accept":             "*/*",
		"Sec-Ch-Ua-Platform": "\"macOS\"",
		"Priority":           "u=1, i",
		"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.6045.199 Safari/537.36",
		"Referer":            "https://toolscord.com/",
		"Sec-Fetch-Site":     "cross-site",
		"Sec-Fetch-Dest":     "empty",
		"Accept-Encoding":    "gzip, deflate, br",
		"Sec-Fetch-Mode":     "cors",
		"Accept-Language":    "en-U",
	}
	data := map[string]interface{}{
		"with_counts": "true",
	}

	// Marshal the map into JSON
	jsonBody, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return
	}

	numConcurrentRequests := 100 // Stable / Good speed --> 750 / 5
	batchSize := 1
	totalRequestLimit := 1000000 // 1000000

	var requestCount int
	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(numConcurrentRequests)

	startTime := time.Now()
	lastUpdateTime := startTime
	lastRequestCount := 0

	for i := 0; i < numConcurrentRequests; i++ {
		go func() {
			defer wg.Done()

			for {
				targetBatch := getNextTargets(&targets, batchSize)
				if len(targetBatch) == 0 {
					return // No more targets to process
				}

				proxy := getRandomProxy(proxies)
				client := createHTTPClient(proxy)
				for _, target := range targetBatch {
					mu.Lock()
					if requestCount >= totalRequestLimit {
						mu.Unlock()
						return // Stop processing requests
					}
					requestCount++
					mu.Unlock()

					targetURL := "https://discord.com/api/v8/invites/" + target
					response := httpRequest(client, targetURL, "GET", jsonBody, headers)
					if response == nil {
						// Proxy error occurred, continue to the next target
						continue
					}

					fmt.Printf("Thread: Code: %d, URL: %s\n", response.StatusCode, target)

					currentTime := time.Now()
					elapsedTime := currentTime.Sub(lastUpdateTime).Seconds()
					if elapsedTime >= 1.0 {
						rps := float64(requestCount-lastRequestCount) / elapsedTime
						setConsoleTitle(fmt.Sprintf("Request Count: %d | RPS: %.2f", requestCount, rps))
						lastRequestCount = requestCount
						lastUpdateTime = currentTime
					}
				}
			}
		}()
	}

	wg.Wait()

}

func createHTTPClient(proxy string) *http.Client {
	proxyParts := strings.Split(proxy, ":")
	if len(proxyParts) != 4 {
		panic("Invalid proxy format: " + proxy)
	}

	proxyURL := "http://" + proxyParts[2] + ":" + proxyParts[3] + "@" + proxyParts[0] + ":" + proxyParts[1]
	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		panic(err)
	}

	transport := &http.Transport{
		Proxy:           http.ProxyURL(proxyURLParsed),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:    100,
		IdleConnTimeout: 30 * time.Second,
	}

	return &http.Client{Transport: transport}
}

func getNextTargets(allTargets *[]string, batchSize int) []string {
	targets := []string{}
	for i := 0; i < batchSize && len(*allTargets) > 0; i++ {
		targets = append(targets, (*allTargets)[0])
		*allTargets = (*allTargets)[1:]
	}
	return targets
}

func getRandomProxy(proxies []string) string {
	return proxies[rand.Intn(len(proxies))]
}

func httpRequest(client *http.Client, targetURL string, method string, data []byte, headers map[string]string) *http.Response {
	request, err := http.NewRequest(method, targetURL, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	for k, v := range headers {
		request.Header.Set(k, v)
	}

	response, err := client.Do(request)
	if err != nil {
		fmt.Printf("Proxy Error: %v\n", err)
		return nil
	}

	body, err := ioutil.ReadAll(response.Body)
	// fmt.Println("Content:", string(body))

	if strings.Contains(string(body), "taken") {
		// Passing
	} else if strings.Contains(string(body), "Available!") { // Username Available

		// Parsing username from url
		rawURL := response.Request.URL.String()
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			fmt.Println("Error:", err)
		}

		queryParams := parsedURL.Query()
		usernameValue := queryParams.Get("username")

		// Sending webhook
		// webhookURL := "https://discord.com/api/webhooks/1171822044514623589/VsxoCd9L3GaqbM5f5oT9Fmmk6ajzk1LxWuGbM-wg6iO426BZmhxAJKzOsgb2d1KKquH9"
		message := "Twitter username available.\n```" + usernameValue + "```"

		fmt.Println(message)
		// err1 := webhook.SendDiscordWebhook(webhookURL, message)
		// if err1 != nil {
		// fmt.Println("Error sending webhook:", err1)
		// }

	} else {
		// Passing
	}

	defer response.Body.Close()
	return response
}

func loadProxies(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var proxies []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxies = append(proxies, scanner.Text())
	}

	return proxies, scanner.Err()
}

func loadTargets(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var targets []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		targets = append(targets, scanner.Text())
	}

	return targets, scanner.Err()
}
