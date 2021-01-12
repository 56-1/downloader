package main

import (
	"bufio"
	"errors"
	"io/ioutil"
	"flag"
	"net/http"
	"sync"
	"os"
)

var (
	wg sync.WaitGroup
	mu sync.Mutex
	RWMu sync.RWMutex
)

var header = http.Header{}
var operate = flag.String("--operate", "down", "select a operation, down or merge")
var url = flag.String("-u", "", "files url")
var file = flag.String("-f", "", "file name")

func main(){
	flag.Parse()

	limit := make(chan int, 10)

	if *operate == "down" {
		//down files
		if *url == "" || *file == "" {
			log.Println("we need the url and file name")
			os.Exit(1)
		}
		lastFile = cleanFile(*url, *file)
		downloadFile(lastFile, limit)
	} else if *operate == "merge" {
		//merge files
		if *file == "" {
			log.Println("we need the file name")
			os.Exit(1)
		}
		merge(*file)
	} else {
		log.Println("you were selected a invalid operation")
	}
}

func cleanFile(url, file string) string {
	path := "./videos/" + file
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		if !os.IsExist(err) {
			log.Println(err.Error())
			os.Exit(1)
		}
	}

	err = os.Chdir(path)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}

func downloadFile(lastFile string, limit chan int){
	f, err := os.Open(lastFile)
	if err != nil {
		log.Println(err.Error())
		return
	}

	input := bufio.NewScanner(f)

	for input.Scan() {
		limit <- 0
		wg.Add()
		go down(input.Text(), limit)
	}

	wg.Wait()
	f.Close()

	return
}

func merge(file string){
	f, err := os.Open(file)
	if err != nil {
		log.Println(err.Error())
		return
	}

	videos, err := os.Create("./videos.avi")
	if err != nil {
		f.Close()
		log.Println(err.Error())
		return
	}

	defer f.Close()
	defer videos.Close()

	input := bufio.NewScanner(f)

	for input.Scan() {
		url := input.Text()
		t := strings.Split(url, "/")
		file := t[len(t)-1]

		data, err := ioutil.ReadFile(file)
		if err != nil {
			log.Println(err.Error())
			return
		}

		_, err = videos.Write(data)
		if err != nil {
			log.Println(err.Error())
			return
		}
	}

	return
}

func down(url string, limit chan int){
	err := core(url)
	if err != nil {
		RWMu.Lock()
		f, err := os.Open("./errMsg.txt")
		if err == nil {
			f.WriteString(url)
			f.Close()
		} else {
			log.Println(err.Error())
		}
		RWMu.Unlock()
	}

	<-limit
	wg.Done()
	return
}

func core(url string) error {
	var (
		resp http.Response
		err error
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header = header

	for i := 0; i < 5; i++ {
		resp, err = http.DefaultClient.Do(req)
		if err != nil && i == 4{return err} else {break}
	}

	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}

	mu.Lock()
	defer mu.Unlock()

	content, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}

	t := strings.Split(url, "/")
	file := t[len(t)-1]

	err = ioutil.WriteFile("./"+file, content, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
