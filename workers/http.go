package workers

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/braintree/manners"
	"github.com/endeveit/go-snippets/cli"
	"github.com/endeveit/go-snippets/config"
	"github.com/gorilla/mux"

	"github.com/endeveit/recause/logger"
	"github.com/endeveit/recause/storage"
)

type WorkerHttp struct {
	addr       string
	maxPerPage int
	maxResults int
	storage    storage.Storage
}

type responseError struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type responseOk struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

// Returns HTTP server object
func NewWorkerHttp(storage storage.Storage) *WorkerHttp {
	addr, err := config.Instance().String("http", "addr")
	cli.CheckError(err)

	maxPerPage, err := config.Instance().Int("http", "max_per_page")
	if err != nil || maxPerPage <= 0 {
		maxPerPage = 100
	}

	maxResults, err := config.Instance().Int("http", "max_results")
	if err != nil || maxPerPage <= 0 {
		maxResults = 1000
	}

	return &WorkerHttp{
		addr:       addr,
		maxPerPage: maxPerPage,
		maxResults: maxResults,
		storage:    storage,
	}
}

// Runs HTTP server
func (wh *WorkerHttp) Run(wg *sync.WaitGroup, die chan bool) {
	defer wg.Done()

	server := manners.NewWithServer(&http.Server{
		Addr:    wh.addr,
		Handler: wh.getRouter(),
	})

	// Start goroutine which will gracefully close server
	go func(server *manners.GracefulServer) {
		for {
			select {
			case <-die:
				logger.Instance().
					Info("Stopping HTTP server")
				server.Close()
				return
			default:
			}

			time.Sleep(time.Second)
		}
	}(server)

	logger.Instance().
		WithField("addr", server.Addr).
		Info("HTTP server started")

	_ = server.ListenAndServe()
}

// Returns mux router
func (wh *WorkerHttp) getRouter() (r *mux.Router) {
	r = mux.NewRouter()
	r.StrictSlash(true)

	r.HandleFunc("/api/dump/{msgId}", wh.handleApiDump)
	r.HandleFunc("/api/search/", wh.handleApiSearch)

	return r
}

// Dumps document
func (wh *WorkerHttp) handleApiDump(w http.ResponseWriter, req *http.Request) {
	msgId := mux.Vars(req)["msgId"]

	if len(msgId) == 0 {
		statusError(w, "Message not found", http.StatusNotFound)

		return
	}

	doc, err := wh.storage.GetMessage(msgId)
	if err != nil {
		logger.Instance().
			WithError(err).
			Warning("Unable to find message")

		statusError(w, "An error occured while getting message", http.StatusInternalServerError)

		return
	}

	statusOk(w, doc)
}

// Handles search request
func (wh *WorkerHttp) handleApiSearch(w http.ResponseWriter, req *http.Request) {
	requestBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Instance().
			WithError(err).
			Warning("Unable to read request body")

		statusError(w, "Request body is empty", http.StatusBadRequest)

		return
	}

	var (
		q             storage.SearchQuery
		requestString string = string(requestBody)
	)

	err = json.Unmarshal(requestBody, &q)
	if err != nil {
		logger.Instance().
			WithError(err).
			WithField("body", requestString).
			Warning("Unable to parse JSON")

		statusError(w, "Provided JSON is invalid", http.StatusBadRequest)

		return
	}

	q.Query = strings.TrimSpace(q.Query)

	if len(q.Query) > 0 {
		err = wh.storage.ValidateQuery(q.Query)
		if err != nil {
			logger.Instance().
				WithError(err).
				WithField("query", q.Query).
				Warning("Unable to validate JSON query")

			statusError(w, "Provided query is invalid", http.StatusBadRequest)

			return
		}
	}

	// Process limit and offset
	if q.Limit <= 0 || q.Limit > wh.maxPerPage {
		q.Limit = wh.maxPerPage
	}

	if q.Offset < 0 {
		q.Offset = 0
	} else if q.Offset+q.Limit > wh.maxResults {
		q.Offset = wh.maxResults - q.Limit
	}

	// Search for messages
	searchResponse, err := wh.storage.GetMessages(&q)
	if err != nil {
		logger.Instance().
			WithError(err).
			WithField("body", requestString).
			Error("Unable to search messages")

		statusError(w, "An error occured while searching messages", http.StatusInternalServerError)

		return
	}

	statusOk(w, searchResponse)
}

// Response with error
func statusError(w http.ResponseWriter, message string, code int) {
	rs := &responseError{
		Status:  "error",
		Message: message,
	}

	addHeaders(w)

	b, err := json.Marshal(rs)
	if err != nil {
		logger.Instance().
			WithField("status", "error").
			Warning("Unable marshal response")
	} else {
		http.Error(w, string(b), code)
	}
}

// Successful response
func statusOk(w http.ResponseWriter, data interface{}) {
	rs := &responseOk{
		Status: "ok",
		Data:   data,
	}

	addHeaders(w)

	b, err := json.Marshal(rs)
	if err != nil {
		logger.Instance().
			WithError(err).
			WithField("status", "ok").
			Warning("Unable to marshal response")
	} else {
		_, err = w.Write(b)
		if err != nil {
			logger.Instance().
				WithError(err).
				Warning("Unable to write response")
		}
	}
}

// Adds some JSON headers to response
func addHeaders(w io.Writer) {
	if headered, ok := w.(http.ResponseWriter); ok {
		headered.Header().Set("Cache-Control", "no-cache")
		headered.Header().Set("Content-type", "application/json")
	}
}
