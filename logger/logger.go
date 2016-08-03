package logger

import (
	"log/syslog"
	"sync"

	log "github.com/Sirupsen/logrus"
	logrus_syslog "github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/endeveit/go-snippets/cli"
	"github.com/endeveit/go-snippets/config"
)

var (
	once   sync.Once
	logger *log.Logger
)

func initLogger() {
	once.Do(func() {
		var (
			level syslog.Priority
			hook  *logrus_syslog.SyslogHook
		)

		proto, err := config.Instance().String("syslog", "proto")
		cli.CheckFatalError(err)

		addr, err := config.Instance().String("syslog", "addr")
		cli.CheckFatalError(err)

		levelName, err := config.Instance().String("syslog", "level")
		cli.CheckFatalError(err)

		switch levelName {
		case "debug":
			level = syslog.LOG_DEBUG
		case "info":
			level = syslog.LOG_INFO
		case "notice":
			level = syslog.LOG_NOTICE
		case "warning":
			level = syslog.LOG_WARNING
		case "err":
			level = syslog.LOG_ERR
		case "crit":
			level = syslog.LOG_CRIT
		case "alert":
			level = syslog.LOG_ALERT
		case "emerg":
			level = syslog.LOG_EMERG
		default:
			level = syslog.LOG_WARNING
		}

		if len(proto) == 0 || len(addr) == 0 {
			writer, err := syslog.New(level, "recause")
			if err != nil {
				cli.CheckFatalError(err)
			}

			hook = &logrus_syslog.SyslogHook{
				Writer: writer,
			}
		} else {
			hook, err = logrus_syslog.NewSyslogHook(proto, addr, level, "recause")
			cli.CheckFatalError(err)
		}

		logger = log.New()
		logger.Hooks.Add(hook)

		log.SetOutput(logger.Writer())
	})
}

// Returns instance of logger object
func Instance() *log.Logger {
	initLogger()

	return logger
}
