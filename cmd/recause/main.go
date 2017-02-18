package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/endeveit/go-snippets/config"
	cc "github.com/urfave/cli"

	"github.com/endeveit/recause/logger"
	"github.com/endeveit/recause/storage"
	"github.com/endeveit/recause/storage/elastic"
	"github.com/endeveit/recause/workers"
)

func main() {
	app := cc.NewApp()

	app.Name = "recause"
	app.Usage = "Simple log management server that receives logs in GELF format"
	app.Version = "0.0.5"
	app.Authors = []cc.Author{
		{
			Name:  "Nikita Vershinin",
			Email: "endeveit@gmail.com",
		},
	}
	app.Flags = []cc.Flag{
		cc.StringFlag{
			Name:  "config, c",
			Value: "/etc/recause/config.cfg",
			Usage: "path to the configuration file",
		},
	}

	app.Action = actionRun

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unhandled error occurred while running application: %v\n", err)
	}
}

func actionRun(c *cc.Context) error {
	_ = config.Instance(c.String("config"))

	var (
		wg          *sync.WaitGroup = &sync.WaitGroup{}
		die         chan bool       = make(chan bool)
		storage     storage.Storage
		workersList []workers.Worker
	)

	// Listen for SIGINT
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		for range ch {
			logger.Instance().Info("Caught interrupt signal")

			// Close all workers.
			close(die)
		}
	}()

	storage = elastic.NewElasticStorage()
	go storage.PeriodicFlush(die)

	workersList = append(workersList, workers.NewWorkerHttp(storage))
	workersList = append(workersList, workers.NewWorkerReceiver(storage))

	wg.Add(len(workersList))

	for _, w := range workersList {
		go w.Run(wg, die)
	}

	wg.Wait()

	return nil
}
