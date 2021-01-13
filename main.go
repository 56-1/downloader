package main

import (
	"bufio"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	count   int
	RWMu    sync.RWMutex
	mu      sync.Mutex
	wg      sync.WaitGroup
	limit   = make(chan int, 20)
	url     = flag.String("u", "http://127.0.0.1:8080/index.html", "url of m3u8 file")
	file    = flag.String("f", "1", "save file name")
	operate = flag.String("o", "down", "select a operation, down or merge")
	header  = http.Header{
		"User-Agent": []string{"Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/75.0.3770.80 Safari/537.36"},
	}
)

func main() {
	flag.Parse()

	/*#########################*/

	if *operate == "merge" {
		merge(*file)
	} else if *operate != "down" {
		log.Println("you were selected a invaild operation")
		os.Exit(1)
	}

	workDir := "./videos/"+*file
	os.RemoveAll(workDir)
	err := os.MkdirAll(workDir, os.ModePerm)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	err = os.Chdir(workDir)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	data := catchM3U8(*url)
	if data == nil {
		os.Exit(1)
	}

	content := string(data)
	if strings.Contains(content, ".m3u8") {
		z := ""
		t := strings.Split(content, "\n")
		for _, v := range t {
			if strings.HasSuffix(v, ".m3u8"){
				z = v
				break
			}
		}

		*url = strings.Replace(*url, "/index.m3u8", "", -1)
		dst := []string{*url}
		t = strings.Split(z, "/")
		for _, v := range t {
			if !strings.Contains(*url, v){
				dst = append(dst, v)
			}
		}
		*url = strings.Join(dst, "/")
		data = catchM3U8(*url)
		if data == nil {
			os.Exit(1)
		}
		content = string(data)
	}

	if !strings.Contains(content, ".ts"){
		log.Println("Don`t get ts file "+ *url)
		os.Exit(1)
	}

	*url = strings.Replace(*url, "/index.m3u8", "", -1)
	
	path := "./"+*file+".m3u8"
	f, err := os.Create(path)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	t := strings.Split(content, "\n")
	for _, v := range t {
		if strings.HasSuffix(v, ".ts"){
			if !strings.HasPrefix(v, "http"){
				dst := []string{*url}
				x := strings.Split(v, "/")
				for _, y := range x {
					if !strings.Contains(*url, y){
						dst = append(dst, y)
					}
				}
				v = strings.Join(dst, "/")
			}
			_, err = f.WriteString(v+"\n")
			if err != nil {
				f.Close()
				log.Println(err.Error())
				os.Exit(1)
			}
		}
	}
	f.Close()

	f, err = os.Open(path)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	input := bufio.NewScanner(f)
	for input.Scan() {
		limit <- 1
		wg.Add(1)
		go down(input.Text(), limit)
	}

	wg.Wait()
	if count != 0 {
		log.Println("Had something happend error")
	} else {
		merge(*file)
	}
	log.Println("Done")
}

func get(url string) ([]byte, error) {
	var (
		resp *http.Response
		err  error
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for i := 0; i < 5; i++ {
		resp, err = http.DefaultClient.Do(req)
		if err != nil && i == 4 {
			return nil, err
		} else {
			break
		}
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, errors.New(resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	resp.Body.Close()

	return data, nil
}

func down(url string, limit chan int) {
	defer func() {
		<-limit
		wg.Done()
	}()

	data, err := get(url)
	if err != nil {
		RWMu.Lock()
		defer RWMu.Unlock()
		count++
		f, err := os.OpenFile("./errMsg.txt", os.O_WRONLY|os.O_APPEND, os.FileMode(0666))
		if err != nil {
			log.Println(err.Error())
			return
		}
		defer f.Close()

		_, err = f.WriteString(url + "\n")
		if err != nil {
			log.Println(err.Error())
		}
		return
	}

	mu.Lock()
	defer mu.Unlock()

	t := strings.Split(url, "/")
	file := "./" + t[len(t)-1]

	err = ioutil.WriteFile(file, data, os.FileMode(0666))
	if err != nil {
		log.Println(err.Error())
	}

	return
}

func catchM3U8(url string) []byte {
	data, err := get(url)
	if err != nil {
		log.Println(err.Error() + "# " + url)
		return nil
	}
	return data
}

func merge(file string){
	err := os.Chdir("./videos/"+file)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	f, err := os.Open("./"+file+".m3u8")
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	videos, err := os.Create("./"+file+".avi")
	if err != nil {
		f.Close()
		log.Println(err.Error())
		os.Exit(1)
	}

	input := bufio.NewScanner(f)
	for input.Scan(){
		t := strings.Split(input.Text(), "/")
		tsFile := t[len(t)-1]
		data, err := ioutil.ReadFile(tsFile)
		if err != nil {
			log.Println(err.Error())
			break
		}
		_, err = videos.Write(data)
		if err != nil {
			log.Println(err.Error())
			break
		}
		err = os.Remove(tsFile)
		if err != nil {
			log.Println(err.Error())
			break
		}
	}

	f.Close()
	videos.Close()
	os.Exit(0)
}
