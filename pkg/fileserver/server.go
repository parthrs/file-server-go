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
	"time"

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

func (s *FileService) upload(w http.ResponseWriter, r *http.Request) {
	fileName := strings.TrimPrefix(r.URL.Path, "/upload/")
	incomingBytes := r.ContentLength
	log.Info().
		Str("fileName", fileName).
		Int("contentLength", int(incomingBytes)).
		Msg("Handling request for file upload")
	filePath := DefaultStoragePath + "/" + fileName // If fileObj is found this goes unused

	if incomingBytes == 0 {
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

	totalReadBytes, totalBytesWritten := 0, 0
	readBuffer := make([]byte, 10000000) // 10MB
	defer r.Body.Close()

	// Read data from body and keep writing to file
	for int64(totalReadBytes) < incomingBytes {
		readBytes, err := io.ReadFull(r.Body, readBuffer)
		if readBytes == 0 {
			log.Error().Err(err).Msg("Unable to read data from request body")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server encountered an exception in reading data from request body"))
			localFile.Close()
			os.Remove(filePath)
			return
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			} else if err.Error() != "unexpected EOF" {
				log.Error().Err(err).Msg("Error reading data from request body")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Server encountered an exception while reading data from request body"))
				localFile.Close()
				os.Remove(filePath)
				return
			}
		}
		totalReadBytes += readBytes
		log.Info().
			Int("readBytes", readBytes).
			Int("totalReadBytes", totalReadBytes).
			Msg("Read bytes into memory")

		writeBytes, err := localFile.Write(readBuffer)
		if err != nil {
			log.Error().Err(err).Msg("Unable to write data to local file")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server encountered an exception while writing data to local file"))
			localFile.Close()
			os.Remove(filePath)
			return
		}
		totalBytesWritten += writeBytes
		log.Info().
			Int("writeBytes", writeBytes).
			Int("totalBytesWritten", totalBytesWritten).
			Msg("Wrote bytes to file")
		time.Sleep(time.Second * 5) // This is just to see the bytes move around
	}

	if totalBytesWritten < int(incomingBytes) {
		log.Error().
			Int("incomingBytes", int(incomingBytes)).
			Int("totalReadBytes", totalReadBytes).
			Int("totalBytesWritten", totalBytesWritten).
			Msg("Total written bytes is less than contenlength")
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
