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
	// p.HTTPServer.ReadTimeout = 120

	return &p, nil
}

func (s *FileService) upload(w http.ResponseWriter, r *http.Request) {
	fileName := strings.TrimPrefix(r.URL.Path, "/upload/")
	log.Info().Str("fileName", fileName).Msg("Handling request for file upload")
	//incomingBytes := r.ContentLength
	r.ParseForm()
	log.Info().
		Any("Content-length", r.ContentLength).
		Any("FormValue", r.FormValue("file")).
		Any("Header", r.Header).
		Msg("")
	filePath := DefaultStoragePath + "/" + fileName // If fileObj is found this goes unused

	fileObj, found := s.DB[fileName]
	var localFile *os.File
	var err error

	if found {
		localFile, err = os.OpenFile(filePath+"-temp", os.O_CREATE|os.O_WRONLY, 0664)
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

	// Currently read full file into memory
	// Read call on the r.Body will read upto
	// len(data)
	data, err := io.ReadAll(r.Body)
	log.Info().Msgf("Read %d bytes", len(data))

	// Empty file uploaded
	if len(data) == 0 {
		log.Error().Err(err).Msg("Empty file being uploaded. Skipping.")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Please upload a non-empty file."))
		localFile.Close()
		os.Remove(filePath)
		return
	}

	// Err in reading data into memory
	if err != nil {
		log.Error().Err(err).Msg("Unable to read uploaded data")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server encountered an exception while reading data"))
		localFile.Close()
		if !found {
			os.Remove(filePath)
		}
		return
	}

	// Write in memory byte to file and handle
	// error
	writeBytes, err := localFile.Write(data)
	if err != nil || writeBytes != int(r.ContentLength) {
		log.Error().Err(err).Msg("Unable to write uploaded data to file")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server encountered an exception while writing data to local file"))
		localFile.Close()
		os.Remove(filePath)
		return
	}

	if found {
		err := os.Rename(filePath, fileObj.File.Name())
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
