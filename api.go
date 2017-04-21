package main

import (
	//"flag"
	//"path"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

var BaseDir = "/home/pbusko/Desktop/Flask_pictures/"
var BaseURL, _ = url.Parse("http://cdn.thomascook.com/")
var PidFile = "/home/pbusko/Desktop/Flask_pictures/api.pid"

type errorType struct {
	value string
}

func (m errorType) Error() string {
	return m.value
}

type UploadedFile struct {
	name string
	url  string
}

type ApiResponse struct {
	images []UploadedFile
}

func (response *ApiResponse) AddFile(file UploadedFile) {
	response.images = append(response.images, file)
}

func savePidFile(pid int) error {
	data := []byte(strconv.Itoa(pid))
	err := ioutil.WriteFile(PidFile, data, 0644)
	if err != nil {
		return err
	}
	return err
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Status success")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case "GET":
		fmt.Fprintf(w, IndexPage)

	case "POST":
		err := r.ParseMultipartForm(200000)
		if err != nil {
			fmt.Fprintln(w, err)
			return
		}

		files := r.MultipartForm.File["file"]
		err = validateMimeType(files)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		for i, _ := range files {
			file, err := files[i].Open()
			defer file.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			out, err := os.Create(BaseDir + files[i].Filename)
			defer out.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			_, err = io.Copy(out, file)

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			fmt.Fprintf(w, "Files uploaded successfully : ")
			fmt.Fprintf(w, files[i].Filename+"\n")

		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func main() {
	err := savePidFile(os.Getpid())
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	f, err := os.OpenFile("cdn-api.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(f)

	srv := http.Server{Addr: "127.0.0.1:8080"}
	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/upload", uploadHandler)

	sig_chan := make(chan os.Signal, 1)
	signal.Notify(sig_chan, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		sigReceived := <-sig_chan
		signal.Stop(sig_chan)
		fmt.Println("Exit command received.", sigReceived)
		srv.Shutdown(nil)
		os.Remove(PidFile)
		os.Exit(0)

	}()
	log.Fatal(srv.ListenAndServe())
}
