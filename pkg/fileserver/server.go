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

// FileObject is a unique reference to a
// file on disk.
// It adds a mutex to avoid concurrent writes.
type FileObject struct {
	Mu   sync.RWMutex
	Path string
}

// FileDB is the in-memory DB used
// by the file server to keep track
// of file references
type FileDB map[string]*FileObject

// NewFileDB returns a new FileDB
func NewFileDB() FileDB {
	return FileDB{}
}

// FileService is a fileserver that can handle
// upload (PUTs) and download (GETs) requests from clients
// over http
type FileService struct {
	DB          FileDB
	HTTPServer  *http.Server
	Port        string
	StoragePath string
}

// NewFileService returns a fileserver to handle requests
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
	mux.HandleFunc("/download/", p.download)

	muxWithLogger := httpRequestLoggerWrapper(mux)

	p.HTTPServer.Addr = ":" + p.Port
	p.HTTPServer.Handler = muxWithLogger

	return &p, nil
}

// upload processes the user file upload for a PUT request
func (s *FileService) upload(w http.ResponseWriter, r *http.Request) {
	// Parse filename from the upload URL
	// curl -T filename.extension http://127.0.0.1:37899/upload/
	// makes curl append filename.extension at the end of the URL
	// Note, that is only possible because of the trailing "/"
	fileName := strings.TrimPrefix(r.URL.Path, "/upload/")
	log.Info().
		Str("fileName", fileName).
		Int("contentLength", int(r.ContentLength)).
		Msg("Processing upload")
	filePath := DefaultStoragePath + "/" + fileName

	// Check for empty file uploads
	if r.ContentLength == 0 {
		log.Error().Msg("Empty file being uploaded. Skipping.")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Please upload a non-empty file."))
		return
	}

	// Check if file already exists
	fileObj, found := s.DB[fileName]
	var localFile *os.File
	var err error

	// If file exists, create a new file with "-temp" suffix
	// once the upload is successful, rename it to the existing
	// file. Lock the mutex on the FileObj in this case.
	// TBD: What if two upload requests come in for a new file
	// in close proximity? It makes sense to lock the first request?
	if found {
		filePath += "-temp"
	} else {
		fileObj = &FileObject{
			Path: filePath,
			Mu:   sync.RWMutex{},
		}
	}

	fileObj.Mu.Lock()
	defer fileObj.Mu.Unlock()

	log.Info().
		Str("filePath", filePath).
		Msg("Opening file for writing")
	localFile, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		log.Error().Err(err).Msg("Unable to create new file object on the server.")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Server encountered an exception creating the file locally (%v)", err)))
		return
	}

	log.Debug().
		Int("fd", int(localFile.Fd())).
		Msg("File descriptor")

	// io.Copy allocates a 32KB buffer by default
	// https://cs.opensource.google/go/go/+/refs/tags/go1.21.6:src/io/io.go;l=419
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

	// Verify if all the bytes were written to disk
	if writtenBytes != r.ContentLength {
		log.Error().
			Msg("Total written bytes is not same as contenlength")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server could not validate all the data written to local file"))
		localFile.Close()
		os.Remove(filePath)
		return
	}

	// Rename the temp file to existing file, overwriting it
	// And update the FileDB reference (since temp file is a new
	// file with a new reference, renaming does not change the pointer
	// to it)
	// If its a new file, create a new FileObj and add DB reference
	// Note: Renaming does not change the MODIFIED timestamp of the
	// file
	if found {
		err := os.Rename(filePath, DefaultStoragePath+"/"+fileName)
		if err != nil {
			log.Error().Err(err).Msg("Unable to rename temp file to final file")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server encountered an exception while comitting data to local file"))
			localFile.Close()
			os.Remove(filePath)
			return
		}
	} else {
		s.DB[fileName] = fileObj
	}

	localFile.Close()
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Upload successful"))
}

func (s *FileService) download(w http.ResponseWriter, r *http.Request) {
	fileName := strings.TrimPrefix(r.URL.Path, "/download/")
	log.Debug().
		Str("fileName", fileName).
		Msg("Processing download")

	fileObj, found := s.DB[fileName]
	if !found {
		log.Debug().
			Msg("No such file found")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("No such file"))
		return
	}

	fi, err := os.Stat(fileObj.Path)
	if err != nil {
		log.Error().Err(err).Msg("Unable to validate file on disk")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server encountered an exception in validating local file object"))
		return
	}

	w.Header().Add("Content-Length", fmt.Sprintf("%d", fi.Size()))

	localFile, err := os.OpenFile(fileObj.Path, os.O_RDONLY, 0664)
	if err != nil {
		log.Error().Err(err).Msg("Unable to open file object on the server for reading.")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Server encountered an exception opening the file locally (%v)", err)))
		return
	}

	log.Debug().
		Int("fd", int(localFile.Fd())).
		Msg("File descriptor")

	bytes, err := io.Copy(w, localFile)
	if err != nil {
		log.Error().Err(err).Msg("Unable to read/write data from disk")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server encountered an exception in processing the download"))
		return
	}

	if bytes != fi.Size() {
		log.Error().Err(err).Msg("Bytes written to response don't match with size on disk")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server encountered an exception in processing data for this request"))
		return
	}
}

// Start starts the fileservice
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

// Stop shutsdown the file service
func (s *FileService) Stop(ctx context.Context) error {
	log.Info().Msg("Stopping server..")
	err := s.HTTPServer.Shutdown(ctx)
	if err != nil {
		log.Err(err).Msg("Error starting the server..")
		return err
	}
	return nil
}

// httpRequestLoggerWrapper is a wrapper around mux
// which logs every request to the server
func httpRequestLoggerWrapper(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.Split(r.URL.Path, "/")
		if !slices.Contains(ignoredPaths, path[1]) {
			log.Info().Msgf("Server received %s request at path %s", r.Method, r.URL.Path)
		}
		h.ServeHTTP(w, r)
	})
}
