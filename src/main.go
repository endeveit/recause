package main

import (
	"os"
	"os/signal"
	"sync"

	cc "github.com/codegangsta/cli"
	hc "github.com/endeveit/go-snippets/cli"
	"github.com/endeveit/go-snippets/config"

	"./recause/logger"
	"./recause/storage"
	"./recause/storage/elastic"
	"./recause/workers"
)

func main() {
	app := cc.NewApp()

	app.Name = "recause"
	app.Usage = "Simple log management server that receives logs in GELF format"
	app.Version = "0.0.2"
	app.Authors = []cc.Author{
		{
			Name:  "Nikita Vershinin",
			Email: "endeveit@gmail.com"},
	}
	app.Email = "endeveit@gmail.com"
	app.Action = actionRun
	app.Flags = []cc.Flag{
		cc.StringFlag{
			Name:  "config, c",
			Value: "/etc/recause/config.cfg",
			Usage: "path to the configuration file",
		},
		cc.StringFlag{
			Name:  "pid, p",
			Value: "/var/run/recause/pid",
			Usage: "Path to the file where PID will be stored",
		},
	}

	app.Run(os.Args)
}

func actionRun(c *cc.Context) {
	_ = config.Instance(c.String("config"))

	var (
		wg          *sync.WaitGroup = &sync.WaitGroup{}
		die         chan bool       = make(chan bool)
		storage     storage.Storage
		workersList []workers.Worker
	)

	pidfile := c.String("pid")
	hc.WritePid(pidfile)

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
	hc.StopExecution(0, pidfile)
}
