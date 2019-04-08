package log

import (
	"bufio"
	"fmt"
	"path"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// NewDevice 创建一个新的日志输出设备
func NewDevice(define string) Device {
	var items = strings.SplitN(define, ":", 2)
	var name = items[0]
	var args string
	if len(items) == 2 {
		args = items[1]
	}
	return deviceMap[name](args)
}

// FileDevice 文件输出设备
type FileDevice struct {
	file     *os.File
	writer   *bufio.Writer
	prefix   string
	lock     sync.Mutex
	lastdate uint32
}

func createFileDevice(args string) Device {
	return &FileDevice{
		prefix: args,
	}
}

func (file *FileDevice) Write(p []byte) {
	var err error
	date := atomic.LoadUint32(&lastDate)
	file.lock.Lock()
	if file.lastdate != date {
		if file.file != nil {
			file.writer.Flush()
			err = file.file.Close()
			if err != nil {
				fmt.Printf("ERROR: logger cannot close file: %v\n", err.Error())
			}
			file.file = nil
		}
	}
	if file.file == nil {
		filename := fmt.Sprintf("%s-%v.log", file.prefix, date)
		dir := path.Dir(filename)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			os.MkdirAll(dir, os.ModePerm)
		}
		file.file, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			file.lock.Unlock()
			fmt.Printf("ERROR: logger cannot open file: %v\n", err.Error())
			return
		}
		file.writer = bufio.NewWriter(file.file)
		file.lastdate = date
	}
	_, err = file.writer.Write(p)
	file.lock.Unlock()
	if err != nil {
		fmt.Printf("ERROR: logger cannot write file: %v\n", err.Error())
	}
	return
}

// Flush 刷新到设备
func (file *FileDevice) Flush() {
	file.lock.Lock()
	if file.writer != nil {
		file.writer.Flush()
	}
	file.lock.Unlock()
}

// ConsoleDevice 控制台设备
type ConsoleDevice struct {
	lock sync.Mutex
}

func createConsoleDevice(args string) Device {
	return &ConsoleDevice{}
}

func (console *ConsoleDevice) Write(p []byte) {
	console.lock.Lock()
	os.Stdout.Write(p)
	console.lock.Unlock()
}

// Flush 无行为
func (console *ConsoleDevice) Flush() {
}

// StdoutDevice 标准输出设备，定时刷新
type StdoutDevice struct {
	writer *bufio.Writer
	lock   sync.Mutex
}

func createStdoutDevice(args string) Device {
	var device = &StdoutDevice{
		writer: bufio.NewWriter(os.Stdout),
	}
	return device
}

// Write 写入
func (console *StdoutDevice) Write(p []byte) {
	console.lock.Lock()
	console.writer.Write(p)
	console.lock.Unlock()
}

// Flush 刷新
func (console *StdoutDevice) Flush() {
	console.lock.Lock()
	console.writer.Flush()
	console.lock.Unlock()
}
