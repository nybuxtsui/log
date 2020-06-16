package log

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	// DEBUG 日志级别
	DEBUG = iota
	// INFO 日志级别
	INFO = iota
	// WARN 日志级别
	WARN = iota
	// ERROR 日志级别
	ERROR = iota
	// DISABLE 日志级别
	DISABLE = iota
	// FATAL 日志级别
	FATAL = iota
)

// Logger 日志对象
type Logger struct {
	minLevel int
	format   Formatter
	writers  []Writer
}

// Writer 日志输出对象
type Writer struct {
	level  int
	device Device
}

// Device 日志输出设备
type Device interface {
	Write(msg []byte)
	Flush()
}

// Formatter 日志格式化接口
type Formatter interface {
	Format(level int, msg string) *bytes.Buffer
}

// LoggerDefine 日志配置
type LoggerDefine struct {
	Name   string `toml:"name"`
	Level  string `toml:"level"`
	Writer string `toml:"writer"`
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Logger []LoggerDefine `toml:"logger"`
}

var (
	lastTime  uint32
	lastDate  uint32
	deviceMap = map[string]func(string) Device{
		"file":    createFileDevice,
		"stdout":  createStdoutDevice,
		"console": createConsoleDevice,
	}

	// defaultLogger = NewLogger(&DefaultFormatter{}, NewWriter(DEBUG, "console"))
	loggerMap = map[string]*Logger{}

	// ErrNameNotFound 日志名称没找到
	ErrNameNotFound = errors.New("name_not_found")
	// ErrIndexOutOfBound 日志索引没找到
	ErrIndexOutOfBound = errors.New("index_out_of_bound")
	bgWorkerCloseCh    = make(chan chan int, 1)
)

func init() {
	go bgWorker()
	Init(nil)
}

func bgWorker() {
	updateNow()
	timer := time.NewTicker(1 * time.Second)
	for {
		select {
		case ch := <-bgWorkerCloseCh:
			timer.Stop()
			ch <- 1
			return
		case <-timer.C:
			updateNow()
			for _, log := range loggerMap {
				log.Flush()
			}
		}
	}
}

// Init 日志库初始化
func Init(config []LoggerDefine) {
	// 关闭bgWorker
	ch := make(chan int, 1)
	bgWorkerCloseCh <- ch
	<-ch

	for _, logger := range config {
		logger.Name = strings.ToLower(logger.Name)
		logger.Writer = strings.ToLower(logger.Writer)
		log, ok := loggerMap[logger.Name]
		if !ok {
			log = NewLogger(&DefaultFormatter{}, NewWriter(getLevelFromStr(logger.Level), logger.Writer))
			loggerMap[logger.Name] = log
		} else {
			log.writers = append(log.writers, NewWriter(getLevelFromStr(logger.Level), logger.Writer))
		}
		log.UpdateLevel()
	}
	// 添加默认日志对象
	if _, ok := loggerMap["default"]; !ok {
		loggerMap["default"] = NewLogger(&DefaultFormatter{}, NewWriter(DEBUG, "console"))
	}
	go bgWorker()
}

// InitFromStr 从字符串初始化
func InitFromStr(tomlstr string) {
	var config LoggerConfig

	var _, err = toml.Decode(tomlstr, &config)
	if err != nil {
		fmt.Printf("ERROR: logger read config: %v\n", err.Error())
		os.Exit(1)
	}
	Init(config.Logger)
}

// InitFromFile 从配置文件初始化
func InitFromFile(configFile string) {
	var tomlstr, err = ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Printf("ERROR: logger read config: %v\n", err.Error())
		os.Exit(1)
	}
	InitFromStr(string(tomlstr))
}

// GetLogger 根据名字获取日志对象
func GetLogger(name string) *Logger {
	var logger, ok = loggerMap[name]
	if ok {
		return logger
	}
	return loggerMap["default"]
}

func getLevelFromStr(level string) int {
	switch strings.ToLower(level) {
	case "d":
		return DEBUG
	case "i":
		return INFO
	case "w":
		return WARN
	case "e":
		return ERROR

	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "warning":
		return WARN
	case "err":
		return ERROR
	case "error":
		return ERROR
	case "disable":
		return DISABLE
	default:
		fmt.Printf("ERROR: logger level unknown: %v\n", level)
		return INFO
	}
}

// SetLevel 设置日志级别
func SetLevel(name string, index int, level string) error {
	var log *Logger
	var l *Logger
	var ok bool
	if l, ok = loggerMap[name]; !ok {
		fmt.Printf("ERROR: log name not found: %v\n", name)
		return ErrNameNotFound
	}
	log = l
	if index >= len(log.writers) {
		fmt.Printf("ERROR: log index exceed: %v, %v\n", len(log.writers), index)
		return ErrIndexOutOfBound
	}
	var newlevel = getLevelFromStr(level)
	if index == -1 {
		for _, writer := range log.writers {
			writer.level = newlevel
		}
	} else {
		log.writers[index].level = newlevel
	}
	log.UpdateLevel()
	return nil
}

// NewLogger 创建新的日志对象
func NewLogger(format Formatter, writers ...Writer) *Logger {
	var logger = Logger{
		format:  format,
		writers: writers,
	}
	logger.UpdateLevel()
	return &logger
}

// NewWriter 创建新的日志输出对象
func NewWriter(level int, device string) Writer {
	return Writer{
		level:  level,
		device: NewDevice(device),
	}
}

// UpdateLevel 更新日志对象的最小输出级别
func (log *Logger) UpdateLevel() {
	log.minLevel = DISABLE
	for _, writer := range log.writers {
		if writer.level < log.minLevel {
			log.minLevel = writer.level
		}
	}
}

func updateNow() {
	t := time.Now()
	dt := uint32(t.Year()%100*10000 + int(t.Month())*100 + t.Day())
	tm := uint32(t.Hour()*10000 + t.Minute()*100 + t.Second())
	atomic.StoreUint32(&lastDate, dt)
	atomic.StoreUint32(&lastTime, tm)
}

// Flush 刷新日志
func (log *Logger) Flush() {
	for _, writer := range log.writers {
		writer.device.Flush()
	}
}

// Write 输出日志
func (log *Logger) Write(level int, format string, a ...interface{}) {
	if level < log.minLevel {
		return
	}
	var msg string
	if len(a) == 0 {
		msg = format
	} else {
		msg = fmt.Sprintf(format, a...)
	}
	buff := log.format.Format(level, msg)
	b := buff.Bytes()
	for _, writer := range log.writers {
		if level >= writer.level {
			writer.device.Write(b)
		}
	}
	buffs.put(buff)
}

// Debug 输出DEBUG级别日志
func Debug(format string, a ...interface{}) {
	loggerMap["default"].Write(DEBUG, format, a...)
}

// Info 输出INFO级别日志
func Info(format string, a ...interface{}) {
	loggerMap["default"].Write(INFO, format, a...)
}

// Warn 输出WARN级别日志
func Warn(format string, a ...interface{}) {
	loggerMap["default"].Write(WARN, format, a...)
}

// Error 输出ERROR级别日志
func Error(format string, a ...interface{}) {
	loggerMap["default"].Write(ERROR, format, a...)
}

// Fatal 输出FATAL级别日志
func Fatal(format string, a ...interface{}) {
	loggerMap["default"].Write(FATAL, format, a...)
	os.Exit(1)
}

// Debug 输出DEBUG级别日志
func (log *Logger) Debug(format string, a ...interface{}) {
	log.Write(DEBUG, format, a...)
}

// Info 输出INFO级别日志
func (log *Logger) Info(format string, a ...interface{}) {
	log.Write(INFO, format, a...)
}

// Warn 输出WARN级别日志
func (log *Logger) Warn(format string, a ...interface{}) {
	log.Write(WARN, format, a...)
}

// Error 输出ERROR级别日志
func (log *Logger) Error(format string, a ...interface{}) {
	log.Write(ERROR, format, a...)
}

// Fatal 输出FATAL级别日志
func (log *Logger) Fatal(format string, a ...interface{}) {
	log.Write(FATAL, format, a...)
	os.Exit(1)
}
