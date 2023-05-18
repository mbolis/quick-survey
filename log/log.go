package log

import (
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
)

type Level logrus.Level

const (
	PanicLevel = Level(logrus.PanicLevel)
	FatalLevel = Level(logrus.FatalLevel)
	ErrorLevel = Level(logrus.ErrorLevel)
	WarnLevel  = Level(logrus.WarnLevel)
	InfoLevel  = Level(logrus.InfoLevel)
	DebugLevel = Level(logrus.DebugLevel)
	TraceLevel = Level(logrus.TraceLevel)
)

var logger *logrus.Logger

func init() {
	logger = logrus.New()
	logger.Formatter = &logrus.TextFormatter{
		DisableLevelTruncation: true,
		PadLevelText:           true,
		TimestampFormat:        "2006/01/02 15:04:05",
		FullTimestamp:          true,
	}
}

func Logger() middleware.LoggerInterface {
	return logger
}

func SetLevel(level Level) {
	logger.Level = logrus.Level(level)
}

func Logf(level Level, fmt string, args ...any) {
	logger.Logf(logrus.Level(level), fmt, args...)
}
func Log(level Level, args ...any) {
	logger.Logln(logrus.Level(level), args...)
}

func Tracef(fmt string, args ...any) {
	logger.Tracef(fmt, args...)
}
func Trace(args ...any) {
	logger.Traceln(args...)
}

func Debugf(fmt string, args ...any) {
	logger.Debugf(fmt, args...)
}
func Debug(args ...any) {
	logger.Debugln(args...)
}

func Infof(fmt string, args ...any) {
	logger.Infof(fmt, args...)
}
func Info(args ...any) {
	logger.Infoln(args...)
}

func Printf(fmt string, args ...any) {
	logger.Printf(fmt, args...)
}
func Print(args ...any) {
	logger.Println(args...)
}

func Warnf(fmt string, args ...any) {
	logger.Warnf(fmt, args...)
}
func Warn(args ...any) {
	logger.Warnln(args...)
}

func Warningf(fmt string, args ...any) {
	logger.Warningf(fmt, args...)
}
func Warning(args ...any) {
	logger.Warningln(args...)
}

func Errorf(fmt string, args ...any) {
	logger.Errorf(fmt, args...)
}
func Error(args ...any) {
	logger.Errorln(args...)
}

func Fatalf(fmt string, args ...any) {
	logger.Fatalf(fmt, args...)
}
func Fatal(args ...any) {
	logger.Fatalln(args...)
}

func Panicf(fmt string, args ...any) {
	logger.Panicf(fmt, args...)
}
func Panic(args ...any) {
	logger.Panicln(args...)
}
