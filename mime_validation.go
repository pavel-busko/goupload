package main

import (
	"mime/multipart"
	"net/http"
)

var allowedMimeTypes []string

func checkMime(m string) bool {
	for _, mi := range allowedMimeTypes {
		if m == mi {
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
		if !checkMime(contentType) {
			err = errorType{"One or more files with forbidden MIME-type received. Aborting"}
			return err
		}

	}
	err = nil
	return err
}
