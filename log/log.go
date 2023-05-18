package log

import "github.com/sirupsen/logrus"

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

var Logger *logrus.Logger

func init() {
	Logger = logrus.New()
	Logger.Formatter = &logrus.TextFormatter{
		DisableLevelTruncation: true,
		PadLevelText:           true,
		TimestampFormat:        "2006/01/02 15:04:05",
		FullTimestamp:          true,
	}
	//Logger.Level = logrus.DebugLevel // TODO
}

func Logf(level Level, fmt string, args ...any) {
	Logger.Logf(logrus.Level(level), fmt, args...)
}
func Log(level Level, args ...any) {
	Logger.Logln(logrus.Level(level), args...)
}

func Tracef(fmt string, args ...any) {
	Logger.Tracef(fmt, args...)
}
func Trace(args ...any) {
	Logger.Traceln(args...)
}

func Debugf(fmt string, args ...any) {
	Logger.Debugf(fmt, args...)
}
func Debug(args ...any) {
	Logger.Debugln(args...)
}

func Infof(fmt string, args ...any) {
	Logger.Infof(fmt, args...)
}
func Info(args ...any) {
	Logger.Infoln(args...)
}

func Printf(fmt string, args ...any) {
	Logger.Printf(fmt, args...)
}
func Print(args ...any) {
	Logger.Println(args...)
}

func Warnf(fmt string, args ...any) {
	Logger.Warnf(fmt, args...)
}
func Warn(args ...any) {
	Logger.Warnln(args...)
}

func Warningf(fmt string, args ...any) {
	Logger.Warningf(fmt, args...)
}
func Warning(args ...any) {
	Logger.Warningln(args...)
}

func Errorf(fmt string, args ...any) {
	Logger.Errorf(fmt, args...)
}
func Error(args ...any) {
	Logger.Errorln(args...)
}

func Fatalf(fmt string, args ...any) {
	Logger.Fatalf(fmt, args...)
}
func Fatal(args ...any) {
	Logger.Fatalln(args...)
}

func Panicf(fmt string, args ...any) {
	Logger.Panicf(fmt, args...)
}
func Panic(args ...any) {
	Logger.Panicln(args...)
}
