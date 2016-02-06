package workers

import (
	"net"
	"sync"
	"time"

	"github.com/endeveit/go-gelf/gelf"
	"github.com/endeveit/go-snippets/cli"
	"github.com/endeveit/go-snippets/config"

	"../logger"
	"../storage"
)

type WorkerReceiver struct {
	storage storage.Storage
	reader  *gelf.Reader
}

// Returns packet receiver object
func NewWorkerReceiver(storage storage.Storage) *WorkerReceiver {
	addr, err := config.Instance().String("receiver", "addr")
	cli.CheckError(err)

	reader, err := gelf.NewReader(addr)
	cli.CheckError(err)

	return &WorkerReceiver{
		storage: storage,
		reader:  reader,
	}
}

// Runs the UDP receiver
func (wr *WorkerReceiver) Run(wg *sync.WaitGroup, die chan bool) {
	var (
		err     error
		message *gelf.Message
	)

	defer wg.Done()

	logger.Instance().
		WithField("addr", wr.reader.Addr()).
		Info("Packet receiver started")

	for {
		select {
		case <-die:
			return
		default:
		}

		// Set read timeout to prevent routine lock
		err = wr.reader.GetConnection().SetDeadline(time.Now().Add(time.Second))
		if err != nil {
			logger.Instance().
				WithError(err).
				Warning("Unable to set timeout")
		}

		message, err = wr.reader.ReadMessage()

		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				logger.Instance().
					WithError(err).
					Debug("Reached timeout, everything is ok")
			} else {
				logger.Instance().
					WithError(err).
					Warning("Unable to read message")
			}

			continue
		}

		go wr.storage.HandleMessage(message)
	}
}
