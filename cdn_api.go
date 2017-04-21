package main

import (
	//	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	//	"os/user"
	//	"path"
)

var BaseDir = "/home/pbusko/Desktop/Flask_pictures/"
var BaseURL, _ = url.Parse("http://cdn.thomascook.com/")

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

		files := r.MultipartForm.File["files"]
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
	var url1, _ = url.Parse("test/folder")
	var url2, _ = url.Parse("/test/folder")
	var url3, _ = url.Parse("/test/folder/")
	var url4, _ = url.Parse("test/folder/")
	fmt.Println(BaseURL.ResolveReference(url1))
	fmt.Println(BaseURL.ResolveReference(url2))
	fmt.Println(BaseURL.ResolveReference(url3))
	fmt.Println(BaseURL.ResolveReference(url4))
	f, err := os.OpenFile("cdn-api.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(f)

	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/upload", uploadHandler)
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", nil))
}
