// -build
package bleve

// Bleve storage will be used when the sorting will be implemented.
import (
	"os"
	"path"
	"sync"
	"time"

	bv "github.com/blevesearch/bleve"
	bvKeywordAnalyzer "github.com/blevesearch/bleve/analysis/analyzers/keyword_analyzer"
	bvStandardAnalyzer "github.com/blevesearch/bleve/analysis/analyzers/standard_analyzer"
	"github.com/endeveit/go-gelf/gelf"
	"github.com/endeveit/go-snippets/cli"
	"github.com/endeveit/go-snippets/config"
	"github.com/satori/go.uuid"

	"../"
	"../../logger"
)

// Structure that used to encapsulate all work with bleve in a single object
type Bleve struct {
	batchSize          int
	index              bv.Index
	messages           []*storage.Message
	mutexHandleMessage *sync.RWMutex
	mutexFlushMessages *sync.RWMutex
	intervalCleanup    time.Duration
	intervalFlush      time.Duration
	lastFlush          time.Time
}

const DOC_TYPE string = "message"

// Returns object to work with bleve
func NewBleveStorage() *Bleve {
	datapath, err := config.Instance().String("bleve", "datapath")
	if err != nil {
		logger.Instance().
			WithError(err).
			Error("Path to bleve index is not provided")

		os.Exit(1)
	}

	dirname := path.Dir(datapath)
	if !cli.FileExists(dirname) {
		logger.Instance().
			WithError(err).
			WithField("directory", dirname).
			Error("Directory with bleve index doesn't exist")

		os.Exit(1)
	}

	batchSize, err := config.Instance().Int("bleve", "batch_size")
	if err != nil {
		batchSize = 10
	}

	var (
		defaultIntervalSecond string = "1s"
		defaultIntervalMonth  string = "720h"
		index                 bv.Index
	)

	intervalCleanupStr, err := config.Instance().String("bleve", "interval_cleanup")
	if err != nil {
		intervalCleanupStr = defaultIntervalMonth
	}

	intervalCleanup, err := time.ParseDuration(intervalCleanupStr)
	if err != nil {
		intervalCleanup, _ = time.ParseDuration(defaultIntervalMonth)
	}

	intervalFlushStr, err := config.Instance().String("bleve", "interval_flush")
	if err != nil {
		intervalFlushStr = defaultIntervalSecond
	}

	intervalFlush, err := time.ParseDuration(intervalFlushStr)
	if err != nil {
		intervalFlush, _ = time.ParseDuration(defaultIntervalSecond)
	}

	if !cli.FileExists(datapath) {
		index, err = bv.New(datapath, getIndexMapping())

		if err != nil {
			logger.Instance().
				WithError(err).
				Error("Unable to create bleve index")

			os.Exit(1)
		} else {
			logger.Instance().
				Debug("New bleve index created")
		}
	} else {
		index, err = bv.Open(datapath)

		if err != nil {
			logger.Instance().
				WithError(err).
				Error("Unable to open bleve index")

			os.Exit(1)
		} else {
			logger.Instance().
				Debug("Bleve index successfully opened")
		}
	}

	return &Bleve{
		batchSize:          batchSize,
		index:              index,
		messages:           []*storage.Message{},
		mutexHandleMessage: &sync.RWMutex{},
		mutexFlushMessages: &sync.RWMutex{},
		intervalCleanup:    -intervalCleanup,
		intervalFlush:      intervalFlush,
		lastFlush:          time.Now(),
	}
}

// Handles GELF-message received through UDP
func (b *Bleve) HandleMessage(msg *gelf.Message) {
	b.mutexHandleMessage.Lock()
	defer b.mutexHandleMessage.Unlock()

	b.messages = append(b.messages, storage.NewMessageFromGelf(msg))
}

// Periodically flushes messages to bleve index
func (b *Bleve) PeriodicFlush(die chan bool) {
	var (
		bvBatchIndex  *bv.Batch
		err           error
		nbMessages    int
		sleepDuration time.Duration = 3 * time.Second
	)

	// Run periodic cleanup task
	go b.periodicCleanup(die)

	for {
		select {
		case <-die:
			return
		default:
		}

		nbMessages = len(b.messages)

		if nbMessages > 0 && (nbMessages >= b.batchSize || time.Now().Sub(b.lastFlush) > b.intervalFlush) {
			b.mutexFlushMessages.Lock()

			bvBatchIndex = b.index.NewBatch()

			for _, message := range b.messages {
				err = bvBatchIndex.Index(uuid.NewV4().String(), message)
				if err != nil {
					logger.Instance().
						WithError(err).
						Warning("Unable to add message to batch")
				}
			}

			if bvBatchIndex.Size() > 0 {
				err = b.index.Batch(bvBatchIndex)

				if err != nil {
					logger.Instance().
						WithError(err).
						Warning("Unable to batch index messages")
				} else {
					logger.Instance().
						WithField("nb_messages", bvBatchIndex.Size()).
						Info("Messages successfully indexed")

					b.lastFlush = time.Now()
					b.messages = []*storage.Message{}
				}
			}

			b.mutexFlushMessages.Unlock()
		}

		time.Sleep(sleepDuration)
	}
}

// Periodically removes old entries from index
func (b *Bleve) periodicCleanup(die chan bool) {
	var (
		bvBatchDelete *bv.Batch
		bvRequest     *bv.SearchRequest
		bvResults     *bv.SearchResult
		bvCleaningNow bool
		bvNbCleaned   int
		err           error
		till          string
		limit         int           = 20
		offset        int           = 0
		sleepDuration time.Duration = 3 * time.Second
	)

	defer b.index.Close()

	for {
		select {
		case <-die:
			return
		default:
		}

		offset = 0
		bvNbCleaned = 0
		bvCleaningNow = true
		till = time.Now().Add(b.intervalCleanup).Format(time.RFC3339)
		bvQuery := bv.NewDateRangeQuery(nil, &till)
		bvQuery.FieldVal = "timestamp"

		for bvCleaningNow != false {
			bvRequest = bv.NewSearchRequestOptions(bvQuery, limit, offset, false)
			bvResults, err = b.index.Search(bvRequest)

			if err != nil {
				logger.Instance().
					WithError(err).
					Warning("Unable to get obsolete messages from index")

				bvCleaningNow = false
				continue
			}

			if bvResults.Hits.Len() == 0 {
				bvCleaningNow = false
				continue
			}

			// List of documents to be deleted
			bvBatchDelete = b.index.NewBatch()
			for _, hit := range bvResults.Hits {
				bvBatchDelete.Delete(hit.ID)
			}

			// Batch delete them
			err = b.index.Batch(bvBatchDelete)
			if err != nil {
				logger.Instance().
					WithError(err).
					Warning("Unable to delete obsolete messages from index")

				bvCleaningNow = false
				continue
			} else {
				bvNbCleaned += bvBatchDelete.Size()
				offset += limit
			}
		}

		if bvNbCleaned > 0 {
			logger.Instance().
				WithField("nb_messages", bvNbCleaned).
				Infof("Obsolete messages were deleted from index")
		}

		time.Sleep(sleepDuration)
	}
}

// Returns data model for index
func getIndexMapping() *bv.IndexMapping {
	indexMapping := bv.NewIndexMapping()
	messageMapping := bv.NewDocumentStaticMapping()

	// Will search exact string, e.g.: «hostname.example.org» will search for «hostname.example.org»
	mappingKeyword := getTextFieldMapping()
	mappingKeyword.Analyzer = bvKeywordAnalyzer.Name

	// Tokenized query, e.g.: «hostname example org» will search for «hostname», «example» or «org»
	mappingText := getTextFieldMapping()
	mappingText.Analyzer = bvStandardAnalyzer.Name

	messageMapping.AddFieldMappingsAt("version", mappingKeyword)
	messageMapping.AddFieldMappingsAt("host", mappingKeyword)
	messageMapping.AddFieldMappingsAt("short_message", mappingText)
	messageMapping.AddFieldMappingsAt("full_message", mappingText)
	messageMapping.AddFieldMappingsAt("timestamp", bv.NewDateTimeFieldMapping())
	messageMapping.AddFieldMappingsAt("level", bv.NewNumericFieldMapping())
	messageMapping.AddFieldMappingsAt("facility", mappingKeyword)
	messageMapping.AddSubDocumentMapping("extra", bv.NewDocumentMapping())

	indexMapping.AddDocumentMapping(DOC_TYPE, messageMapping)

	return indexMapping
}

// Returns text field mapping with disabled term vectors
func getTextFieldMapping() *bv.FieldMapping {
	return &bv.FieldMapping{
		Type:               "text",
		Store:              true,
		Index:              true,
		IncludeTermVectors: false,
		IncludeInAll:       true,
	}
}
