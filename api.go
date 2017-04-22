package main

import (
	"flag"
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

var BaseDir string
var BaseURL *url.URL
var PidFile string
var LogFile string
var ListenAddress string

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

func init() {
	var ListenPort string
	var ListenIp string
	var u string
	flag.StringVar(&BaseDir, "upload-dir", "", "Base dir for uploaded files to save")
	flag.StringVar(&u, "url", "", "Base url for access links.")
	flag.StringVar(&PidFile, "pid-file", "", "Path to pid file.")
	flag.StringVar(&LogFile, "log-file", "", "Path to log file.")
	flag.StringVar(&ListenPort, "port", "8080", "Port to listen.")
	flag.StringVar(&ListenIp, "host", "127.0.0.1", "Host address to listen.")
	flag.Parse()
	BaseURL, _ = url.Parse(u)
	ListenAddress = ListenIp + ":" + ListenPort
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
		//response := ApiResponse{}

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

	f, err := os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(f)

	srv := http.Server{Addr: ListenAddress}

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

	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/upload", uploadHandler)
	log.Fatal(srv.ListenAndServe())
}
