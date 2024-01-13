package fileserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

var (
	DefaultStoragePath = "files"
	ignoredPaths       = []string{}
)

type FileObject struct {
	File *os.File
	Mu   sync.RWMutex
}

type FileDB map[string]*FileObject

func NewFileDB() FileDB {
	return FileDB{}
}

// FileService is a fileserver that can handle
// upload and download requests from clients
// over http
type FileService struct {
	DB          FileDB
	HTTPServer  *http.Server
	Port        string
	StoragePath string
}

func NewFileService() (*FileService, error) {
	if err := os.Mkdir(DefaultStoragePath, 0774); err != nil && err.Error() != "mkdir files: file exists" {
		log.Error().Err(err).Msg("Unable to create local file storage dir. Exiting..")
		return nil, err
	}

	mux := http.NewServeMux()
	p := FileService{
		DB:          NewFileDB(),
		HTTPServer:  &http.Server{},
		Port:        "37899",
		StoragePath: DefaultStoragePath,
	}

	mux.HandleFunc("/upload/", p.upload)
	//mux.HandleFunc("/download", p.download)

	muxWithLogger := httpRequestLoggerWrapper(mux)

	p.HTTPServer.Addr = ":" + p.Port
	p.HTTPServer.Handler = muxWithLogger

	return &p, nil
}

// upload processes the user file upload for a PUT request
func (s *FileService) upload(w http.ResponseWriter, r *http.Request) {
	fileName := strings.TrimPrefix(r.URL.Path, "/upload/")
	log.Info().
		Str("fileName", fileName).
		Int("contentLength", int(r.ContentLength)).
		Msg("Processing upload")
	filePath := DefaultStoragePath + "/" + fileName

	if r.ContentLength == 0 {
		log.Error().Msg("Empty file being uploaded. Skipping.")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Please upload a non-empty file."))
		return
	}

	fileObj, found := s.DB[fileName]
	var localFile *os.File
	var err error

	if found {
		localFile, err = os.OpenFile(filePath+"-temp", os.O_CREATE|os.O_WRONLY, 0664)
		filePath += "-temp"
		fileObj.Mu.Lock()
		defer fileObj.Mu.Unlock()
	} else {
		localFile, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0664)
	}

	if err != nil {
		log.Error().Err(err).Msg("Unable to create new file object on the server.")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Server encountered an exception creating the file locally (%v)", err)))
		return
	}

	writtenBytes, err := io.Copy(localFile, r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Unable error trying to read/write data to disk")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server encountered an exception in processing the upload"))
		localFile.Close()
		os.Remove(filePath)
		return
	}

	log.Info().
		Int64("writtenBytes", writtenBytes).
		Msg("Wrote bytes to file")

	if writtenBytes != r.ContentLength {
		log.Error().
			Msg("Total written bytes is not same as contenlength")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server could not validate all the data written to local file"))
		localFile.Close()
		os.Remove(filePath)
		return
	}

	if found {
		err := os.Rename(filePath, fileObj.File.Name())
		s.DB[fileName].File = localFile
		if err != nil {
			log.Error().Err(err).Msg("Unable to rename temp file to final file")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server encountered an exception while comitting data to local file"))
			localFile.Close()
			os.Remove(filePath)
		}
	} else {
		fileObj = &FileObject{
			File: localFile,
			Mu:   sync.RWMutex{},
		}
		s.DB[fileName] = fileObj
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Upload successful"))

	fileObj.File.Close()
}

// func (s *FileService) download(w http.ResponseWriter, r *http.Request) {

// }

func (s *FileService) Start() error {
	log.Info().Str("Port", s.Port).Msg("Starting server..")
	var err error
	go func() {
		err = s.HTTPServer.ListenAndServe()
	}()
	if err != nil {
		log.Err(err).Msg("Error starting the server..")
		return err
	}
	return nil
}

func (s *FileService) Stop(ctx context.Context) error {
	log.Info().Msg("Stopping server..")
	err := s.HTTPServer.Shutdown(ctx)
	if err != nil {
		log.Err(err).Msg("Error starting the server..")
		return err
	}
	return nil
}

func httpRequestLoggerWrapper(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.Split(r.URL.Path, "/")
		if !slices.Contains(ignoredPaths, path[1]) {
			log.Info().Msgf("Server received %s request at path %s", r.Method, r.URL.Path)
		}
		h.ServeHTTP(w, r)
	})
}
