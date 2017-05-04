package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var BaseDir string
var BaseURL *url.URL
var IndexPage string
var AllowedMimeTypes []string
var Listener net.Listener
var SocketType string
var Socket string
var UploadUrl string
var StatusUrl string
var Pfile string
var PathRegexp *regexp.Regexp
var ErrLogger *log.Logger
var OutLogger *log.Logger

type errorType struct {
	Value string `json:"error"`
}

func (m errorType) Error() string {
	return m.Value
}

type UploadedFile struct {
	Name string `json:"name"`
	Url  string `json:"url"`
}

type ApiResponse struct {
	Images []UploadedFile `json:"images"`
}

func (response *ApiResponse) AddFile(file UploadedFile) {
	response.Images = append(response.Images, file)
}

func init() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage:", filepath.Base(os.Args[0]), "/config/folder")
		os.Exit(1)
	}

	ErrLogger = log.New(os.Stderr, "", log.LstdFlags)
	OutLogger = log.New(os.Stdout, "", log.LstdFlags)

	viper.SetConfigName("api")
	viper.AddConfigPath(filepath.Base(os.Args[1]))
	err := viper.ReadInConfig()
	if err != nil {
		ErrLogger.Fatal(err)
	}

	PathRegexp, _ = regexp.Compile("^/?(?:[0-9a-zA-Z_\\-\\.]+/?)+?$")
	BaseDir = viper.GetString("upload.path")
	BaseURL, err = url.Parse(viper.GetString("http.base_url"))
	if err != nil {
		ErrLogger.Fatal(err)
	}
	IndexPage = viper.GetString("http.index_page")
	AllowedMimeTypes = strings.Split(viper.GetString("upload.mime_types"), ";")
	UploadUrl = viper.GetString("http.upload_url")
	StatusUrl = viper.GetString("http.status_url")
	Pfile = viper.GetString("base.pid_file")
	SocketType = viper.GetString("base.socket_type")

	if SocketType == "tcp" {
		Socket = viper.GetString("base.tcp_socket")
		Listener, err = net.Listen("tcp", Socket)
		if err != nil {
			ErrLogger.Fatal(err)
		}
	} else if SocketType == "unix" {
		Socket = viper.GetString("base.unix_socket")
		if _, err = os.Stat(Socket); err == nil {
			err = os.Remove(Socket)
			if err != nil {
				ErrLogger.Fatal(err)
			}
		}
		Listener, err = net.Listen("unix", Socket)
		if err != nil {
			ErrLogger.Fatal(err)
		}
	} else {
		ErrLogger.Fatal("Unknown socket type, check your config.")
	}
}

func savePidFile(pid int) error {
	data := []byte(strconv.Itoa(pid))
	f, err := os.OpenFile(Pfile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return err
}

func checkMime(m *string) bool {
	for _, mi := range AllowedMimeTypes {
		if *m == mi {
			return true
		}
	}
	return false
}

func validateMimeType(f []*multipart.FileHeader) (err error) {
	mime_buffer := make([]byte, 512)

	for i, _ := range f {
		file, err := f[i].Open()
		defer file.Close()

		_, err = file.Read(mime_buffer)
		if err != nil {
			return err
		}
		file.Seek(0, 0)
		contentType := http.DetectContentType(mime_buffer)
		if !checkMime(&contentType) {
			err = &errorType{"One or more files with forbidden MIME-type received, aborted."}
			return err
		}

	}
	err = nil
	return err
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "{\"status\": \"running\"}")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {

	case "GET":
		fmt.Fprintf(w, IndexPage)

	case "POST":
		rawResponse := ApiResponse{}

		err := r.ParseMultipartForm(200000)
		if err != nil {
			ErrLogger.Print(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		files, exists := r.MultipartForm.File["file"]
		if !exists {
			ErrLogger.Print("No files received in request, aborted.")
			http.Error(w, "Bad Request.\nNo files provided.", http.StatusBadRequest)
			return
		}

		err = validateMimeType(files)
		if err != nil {
			ErrLogger.Print(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fpath, exists := r.MultipartForm.Value["path"]
		if !exists {
			fpath = []string{""}
		} else if valid := PathRegexp.MatchString(fpath[0]); fpath[0] != "" && !valid {
			ErrLogger.Print("Invalid path provided, aborted.")
			http.Error(w, "Bad Request.\nInvalid path provided.", http.StatusBadRequest)
			return
		}

		uplPath := filepath.Join(BaseDir, fpath[0])
		err = os.MkdirAll(uplPath, 0775)
		if err != nil {
			ErrLogger.Print(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		for i, _ := range files {
			file, err := files[i].Open()
			defer file.Close()
			if err != nil {
				ErrLogger.Print(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			fname := filepath.Base(files[i].Filename)
			fmt.Println(fname)
			out, err := os.Create(filepath.Join(uplPath, fname))
			defer out.Close()
			if err != nil {
				ErrLogger.Print(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			_, err = io.Copy(out, file)
			if err != nil {
				ErrLogger.Print(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			frelUrl, err := url.Parse(filepath.Join(fpath[0], fname))
			if err != nil {
				ErrLogger.Print(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			accessUrl := BaseURL.ResolveReference(frelUrl)

			rawResponse.AddFile(UploadedFile{Name: fname, Url: accessUrl.String()})
		}
		response, err := json.Marshal(rawResponse)
		if err != nil {
			ErrLogger.Print(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.Write(response)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func main() {
	err := savePidFile(os.Getpid())
	if err != nil {
		ErrLogger.Fatal(err)
	}

	srv := &http.Server{}
	OutLogger.Println("Server started, listener at:", SocketType, Socket)

	sig_chan := make(chan os.Signal, 1)
	signal.Notify(sig_chan, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func() {
		sigReceived := <-sig_chan
		signal.Stop(sig_chan)
		OutLogger.Println("Exit command received.", sigReceived)
		srv.Shutdown(nil)
		os.Remove(Pfile)
		os.Exit(0)
	}()

	http.HandleFunc(StatusUrl, statusHandler)
	http.HandleFunc(UploadUrl, uploadHandler)
	ErrLogger.Fatal(srv.Serve(Listener))
}
