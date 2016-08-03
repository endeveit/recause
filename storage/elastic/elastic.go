package elastic

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/endeveit/go-gelf/gelf"
	"github.com/endeveit/go-snippets/config"
	"github.com/satori/go.uuid"
	es "gopkg.in/olivere/elastic.v3"
	"gopkg.in/olivere/elastic.v3/uritemplates"

	"recause/logger"
	"recause/storage"
)

type Elastic struct {
	batchSize          int
	indexName          string
	typeName           string
	client             *es.Client
	mutexHandleMessage *sync.RWMutex
	mutexFlushMessages *sync.RWMutex
	ttl                int64
	messages           []*storage.Message
	intervalFlush      time.Duration
	lastFlush          time.Time
}

type validateResult struct {
	Valid bool `json:"valid"`
}

// Returns object to work with bleve
func NewElasticStorage() *Elastic {
	url, err := config.Instance().String("elastic", "url")
	if err != nil {
		logger.Instance().
			WithError(err).
			Error("Elastic url is not provided")

		os.Exit(1)
	}

	client, err := es.NewClient(
		es.SetURL(url),
		es.SetSniff(false),
		es.SetHealthcheck(false),
		es.SetMaxRetries(0),
	)
	if err != nil {
		logger.Instance().
			WithError(err).
			Error("Unable to create client to elastic")

		os.Exit(1)
	}

	indexName, err := config.Instance().String("elastic", "index")
	if err != nil {
		logger.Instance().
			WithError(err).
			Error("Index name is not provided")

		os.Exit(1)
	}

	typeName, err := config.Instance().String("elastic", "type")
	if err != nil {
		logger.Instance().
			WithError(err).
			Error("Type name is not provided")

		os.Exit(1)
	}

	batchSize, err := config.Instance().Int("elastic", "batch_size")
	if err != nil {
		batchSize = 10
	}

	var (
		defaultIntervalSecond string = "1s"
		defaultIntervalMonth  string = "720h"
	)

	intervalCleanupStr, err := config.Instance().String("elastic", "interval_cleanup")
	if err != nil {
		intervalCleanupStr = defaultIntervalMonth
	}

	intervalCleanup, err := time.ParseDuration(intervalCleanupStr)
	if err != nil {
		intervalCleanup, _ = time.ParseDuration(defaultIntervalMonth)
	}

	intervalFlushStr, err := config.Instance().String("elastic", "interval_flush")
	if err != nil {
		intervalFlushStr = defaultIntervalSecond
	}

	intervalFlush, err := time.ParseDuration(intervalFlushStr)
	if err != nil {
		intervalFlush, _ = time.ParseDuration(defaultIntervalSecond)
	}

	return &Elastic{
		batchSize:          batchSize,
		indexName:          indexName,
		typeName:           typeName,
		client:             client,
		messages:           []*storage.Message{},
		mutexHandleMessage: &sync.RWMutex{},
		mutexFlushMessages: &sync.RWMutex{},
		ttl:                int64(intervalCleanup.Seconds() * 1000), // TTL is in milliseconds
		intervalFlush:      intervalFlush,
		lastFlush:          time.Now(),
	}
}

// Returns message from elastic index
func (e *Elastic) GetMessage(msgId string) (doc map[string]interface{}, err error) {
	response, err := e.client.
		Get().
		Index(e.indexName).
		Type(e.typeName).
		Id(msgId).
		Do()

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(*response.Source, &doc)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// Searches for mssages
func (e *Elastic) GetMessages(q *storage.SearchQuery) (result *storage.SearchResult, err error) {
	var (
		filters []es.Query    = []es.Query{}
		query   *es.BoolQuery = es.NewBoolQuery()
		tsRange *es.RangeQuery
		msg     *storage.Message
	)

	if len(q.Query) > 0 {
		filters = append(filters, es.NewQueryStringQuery(q.Query))
	} else {
		filters = append(filters, es.NewMatchAllQuery())
	}

	if q.From.Second() > 0 && q.To.Second() > 0 {
		tsRange = es.NewRangeQuery("timestamp")
		tsRange = tsRange.From(q.From)
		tsRange = tsRange.To(q.To)
		filters = append(filters, tsRange)
	} else if q.From.Second() > 0 && q.To.Second() <= 0 {
		tsRange = es.NewRangeQuery("timestamp")
		tsRange = tsRange.From(q.From)
		filters = append(filters, tsRange)
	} else if q.To.Second() > 0 && q.From.Second() <= 0 {
		tsRange = es.NewRangeQuery("timestamp")
		tsRange = tsRange.To(q.To)
		filters = append(filters, tsRange)
	}

	rs, err := e.client.
		Search(e.indexName).
		Type(e.typeName).
		Query(query.Filter(filters...)).
		Sort("timestamp", false).
		From(q.Offset).
		Size(q.Limit).
		Do()

	if err != nil {
		return nil, err
	}

	result = &storage.SearchResult{
		Total:    rs.TotalHits(),
		TookMs:   rs.TookInMillis,
		Limit:    q.Limit,
		Offset:   q.Offset,
		Messages: []storage.Message{},
	}

	// rs.Each() is not used because we need to add message id manually
	if rs.Hits != nil && rs.Hits.Hits != nil && len(rs.Hits.Hits) > 0 {
		for _, hit := range rs.Hits.Hits {
			msg = new(storage.Message)
			err = json.Unmarshal(*hit.Source, msg)

			if err != nil {
				continue
			}

			msg.Id = hit.Id

			result.Messages = append(result.Messages, *msg)
		}
	}

	return result, nil
}

// Handles GELF-message received through UDP
func (e *Elastic) HandleMessage(msg *gelf.Message) {
	e.mutexHandleMessage.Lock()
	defer e.mutexHandleMessage.Unlock()

	e.messages = append(e.messages, storage.NewMessageFromGelf(msg))
}

// Periodically flushes messages to elastic
func (e *Elastic) PeriodicFlush(die chan bool) {
	var (
		esBulk        *es.BulkService
		esResponse    *es.BulkResponse
		err           error
		nbMessages    int
		sleepDuration time.Duration = 3 * time.Second
	)

	for {
		select {
		case <-die:
			return
		default:
		}

		nbMessages = len(e.messages)

		if nbMessages > 0 && (nbMessages >= e.batchSize || time.Now().Sub(e.lastFlush) > e.intervalFlush) {
			e.mutexFlushMessages.Lock()

			esBulk = e.client.Bulk()

			for _, message := range e.messages {
				esBulk.Add(es.NewBulkIndexRequest().
					Index(e.indexName).
					Type(e.typeName).
					Id(uuid.NewV4().String()).
					Ttl(e.ttl).
					Doc(message))
			}

			if esBulk.NumberOfActions() > 0 {
				esResponse, err = esBulk.Do()

				if err != nil {
					logger.Instance().
						WithError(err).
						Warning("Unable to batch index messages")
				} else {
					nbCreated := len(esResponse.Indexed())
					if nbCreated != nbMessages {
						logger.Instance().
							WithField("nb_messages", nbMessages).
							WithField("nb_created", nbCreated).
							Warning("Not all messages were indexed")
					} else {
						logger.Instance().
							WithField("nb_messages", nbMessages).
							Info("Messages successfully indexed")
					}

					e.lastFlush = time.Now()
					e.messages = []*storage.Message{}
				}
			}

			e.mutexFlushMessages.Unlock()
		}

		time.Sleep(sleepDuration)
	}
}

// Validates search query
func (e *Elastic) ValidateQuery(query string) (err error) {
	path, err := uritemplates.Expand("/{index}/{type}/_validate/query", map[string]string{
		"index": e.indexName,
		"type":  e.typeName,
	})

	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("q", query)

	rs, err := e.client.PerformRequest("GET", path, params, nil)
	if err != nil {
		return err
	}

	var result validateResult

	err = json.Unmarshal(rs.Body, &result)
	if err != nil {
		return err
	}

	if !result.Valid {
		return errors.New("Provided query is invalid")
	}

	return nil
}
