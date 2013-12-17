package glog

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	//specific, used for close
	SIGNAL_CLOSE = 0

	//log level
	ERROR   = 1
	WARNING = 2
	NOTICE  = 4
	INFO    = 8
	DEBUG   = 16

	//split mode
	SPLIT_DAY  = 1 // like "day"
	SPLIT_HOUR = 2 // like "5hour"
	SPLIT_MIN  = 3 // like "15min"
	SPLIT_SIZE = 9 // like "100m"

)

const (
	//default logChan's buffer
	CHAN_BUFFER_SIZE = 1024

	//Default of writeBuf, user can re-custom the size,
	//if too small, and has a lot of logMsg, channel may be blocked
	DEFAULT_BUFFER_SIZE = 10240

	//private use
	LEVEL_COUNT = 5
)

//struct of logLevel
type Level byte

//struct of Log
type Log struct {
	//public
	Buf       int //if too small, will blocking the write log
	FlushTime int
	Split     string //split log file by "day", "hour", "15min" or "100m"
	Level     byte   //level mode, example: "ERROR"+"WARNING"+"NOTICE" = 1+2+4 = 7
	FileMode  os.FileMode
	//private
	isClosed  bool
	levelConf map[Level]LevelConf
}

//struct of Level Config
type LevelConf struct {
	Level    Level
	IsPrint  bool //is print to STDOUT
	FileName string
}

//struct of LogMessage
type logMsg struct {
	level Level
	str   string
}

type split struct {
	id    byte
	value int
}

//runtime struct
type runtime struct {
	fileName        map[Level]string
	files           map[string]*os.File
	bufsCounter     map[string]int
	fileSizeCounter map[string]int64
	writeBufs       map[string][]string
	split           split
}

//conver logLevel to strings
func (t Level) String() string {
	return LevelToStr[byte(t)]
}

//logLevel mapping to strings
var LevelToStr = map[byte]string{
	ERROR:   "ERROR",
	WARNING: "WARNING",
	NOTICE:  "NOTICE",
	INFO:    "INFO",
	DEBUG:   "DEBUG",
}

//log channel, ticker
var (
	logChan        chan logMsg  //worker channel
	logFlushTicker *time.Ticker //flush ticker
)

//global variable
var glog *Log

var gruntime = runtime{}

/**
 * useage:
 *	new()  use default config
 *  new(log Log) use user custom config but default level config
 *  new(log Log, []LogLevelConf)  use user custom config
 *  new(log Log, type1 LogLevelConf, type2 LogLevelConf...)  Same as above
 */
func New(args ...interface{}) (*Log, error) {
	var err error
	//existed Log
	if glog != nil {
		return glog, nil
	}

	var (
		//default Level Config
		levelConf = map[Level]LevelConf{
			ERROR:   LevelConf{Level: Level(ERROR), IsPrint: true, FileName: ""},
			WARNING: LevelConf{Level: Level(WARNING), IsPrint: true, FileName: ""},
			NOTICE:  LevelConf{Level: Level(NOTICE), IsPrint: true, FileName: ""},
			INFO:    LevelConf{Level: Level(INFO), IsPrint: true, FileName: ""},
			DEBUG:   LevelConf{Level: Level(DEBUG), IsPrint: true, FileName: ""},
		}

		//default Log
		log = Log{
			Buf:       DEFAULT_BUFFER_SIZE,
			FlushTime: 1000,
			Split:     "day", //split log file by "day", "hour", "10min" minutes or "10m" bytes
			Level:     7,     // WARNING + ERROR + NOTICE
			FileMode:  0400,  //permission bits, see os.FileMode
			levelConf: levelConf,
			isClosed:  false,
		}
	)

	//has user custom args
	if len(args) > 0 {
		switch args[0].(type) {
		case Log:
			//set log of custom
			l := args[0].(Log)

			log.Buf, log.FlushTime, log.Split, log.Level, log.FileMode = l.Buf, l.FlushTime, l.Split, l.Level, l.FileMode

			for t, c := range levelConf {
				if _, ok := log.levelConf[t]; !ok {
					log.levelConf[t] = c
				}
			}

			//set level Config of custom
			for i, arg := range args[1:] {
				switch arg.(type) {
				//if only args[1], judge []LogLevelConf
				case map[Level]LevelConf:
					for t, c := range args[1].(map[Level]LevelConf) {
						log.levelConf[t] = c
					}
				//else, the args of each part as a level of a config
				case LevelConf:
					c := arg.(LevelConf)
					log.levelConf[c.Level] = arg.(LevelConf)
				default:
					err = errors.New(fmt.Sprintf("arg[%d] error, must be type of LogLevelConf", i))
					break
				}
			}

		default:
			err = errors.New("args[0] error, must be struct of Log")
		}
	}

	//start LogChan
	if log.Buf < CHAN_BUFFER_SIZE { //set min buf
		log.Buf = CHAN_BUFFER_SIZE
	}

	logChan = make(chan logMsg, CHAN_BUFFER_SIZE)
	// run LogChan worker process
	go runLogChan()
	// start ticker for flush
	logFlushTicker = time.NewTicker(time.Duration(log.FlushTime) * time.Millisecond)
	glog = &log
	return glog, err
}

/**
 *  no block write log, if set the Log.Buf
 *  Usage like:
 *	log.Error("err msg") or log.Warn("warn msg")
 */
func (log *Log) Error(logStr string, args ...interface{}) {
	log.write(ERROR, &logStr, args...)

}

func (log *Log) Warning(logStr string, args ...interface{}) {
	log.write(WARNING, &logStr, args...)

}

func (log *Log) Notice(logStr string, args ...interface{}) {
	log.write(NOTICE, &logStr, args...)
}

func (log *Log) Info(logStr string, args ...interface{}) {
	log.write(INFO, &logStr, args...)
}

func (log *Log) Debug(logStr string, args ...interface{}) {
	log.write(DEBUG, &logStr, args...)
}

/**
 * Blocking when closed, waiting for flush buffers to disk file
 */
func (log *Log) Close() {
	logChan <- logMsg{level: SIGNAL_CLOSE, str: ""}
	for !log.isClosed {
		time.Sleep(10 * time.Millisecond)
	}
	close(logChan)
	glog = nil
}

/**
 * write to LogChan
 */
func (log *Log) write(level Level, logStr *string, args ...interface{}) {

	//Skip unnecessary level
	if (byte(level) & (*log).Level) != byte(level) {
		return
	}

	if len(args) > 0 {
		*logStr = fmt.Sprintf(*logStr, args...)
	}

	//format logStr like 'Error 06/01/02 15:04:05  balabalabala'
	logMsg := logMsg{level: level, str: format(level.String(), *logStr)}

	//print log to stdout
	if glog.levelConf[level].IsPrint {
		fmt.Print(logMsg.str)
	}

	//if no define logfile, return
	if glog.levelConf[level].FileName == "" {
		return
	}

	logChan <- logMsg
	return
}

//format logStr like 'Error 06/01/02 15:04:05.67823  balabalabala'
func format(level string, logStr string) string {
	return fmt.Sprintf("%-7s\t%s\t%s\n", level, time.Now().Format("06/01/02 15:04:05"), logStr)
}

/**
 * worker on the goroutine
 */
func runLogChan() {

	if splitId, splitValue, err := parse_split(glog.Split); err != nil {
		panic(fmt.Sprintf("glog:split config error:%s", err.Error()))
	} else {
		gruntime.split = split{id: splitId, value: splitValue}
	}

	//init glog runtime
	initRuntime()

	//close all open files
	defer func() {
		flushAll()
		for _, fd := range gruntime.files {
			if fd != nil {
				fd.Close()
			}
		}
		glog.isClosed = true
	}()

	//merge logfiles and init
	for _, levelConf := range glog.levelConf {
		fileName := levelConf.FileName
		level := levelConf.Level
		gruntime.fileName[level] = fileName
		gruntime.files[fileName] = nil
		gruntime.writeBufs[fileName] = make([]string, glog.Buf)
		gruntime.bufsCounter[fileName] = 0
	}

	//wait log
L: // label "L" for break
	for {
		select {
		//log
		case logMsg := <-logChan:
			level, logStr := logMsg.level, logMsg.str

			//if closed, flush all
			if level == SIGNAL_CLOSE {
				break L
			}

			fileName := gruntime.fileName[Level(level)]
			bufIndex := gruntime.bufsCounter[fileName]

			//write to buf
			gruntime.writeBufs[fileName][bufIndex] = logStr
			gruntime.bufsCounter[fileName]++
			gruntime.fileSizeCounter[fileName] += int64(len(logStr))

			//writeBuf full, flush now
			if (bufIndex >= (glog.Buf - 1)) || (gruntime.split.id == SPLIT_SIZE && gruntime.fileSizeCounter[fileName] > int64(gruntime.split.value*1024*1024)) {
				fd, err := getFile(fileName)
				if fd == nil || err != nil {
					panic(fmt.Sprintf("glog:create file error:%s", err.Error()))
				}
				flush(fileName, fd)
				//gruntime.writeBufs[fileName] = make([]string, glog.Buf)
				gruntime.bufsCounter[fileName] = 0
				if gruntime.split.id == SPLIT_SIZE {
					gruntime.fileSizeCounter[fileName] = 0
				}
			}

		//flush all
		case <-logFlushTicker.C:
			flushAll()
		}
	}
}

func initRuntime() {
	gruntime.fileName = make(map[Level]string, LEVEL_COUNT)
	gruntime.files = make(map[string]*os.File, LEVEL_COUNT)
	gruntime.writeBufs = make(map[string][]string, LEVEL_COUNT)
	gruntime.bufsCounter = make(map[string]int, LEVEL_COUNT)
	gruntime.fileSizeCounter = make(map[string]int64, LEVEL_COUNT)
}

/**
 * flush writeBuffer to disk
 */
func flush(fileName string, fd *os.File) {
	for i := 0; i < gruntime.bufsCounter[fileName]; i++ {
		fd.WriteString(gruntime.writeBufs[fileName][i])
	}
	fd.Sync()
}

func flushAll() {
	for fileName, index := range gruntime.bufsCounter {
		if index > 0 && fileName != "" {
			fd, err := getFile(fileName)
			if fd != nil && err == nil {
				flush(fileName, fd)
			} else {
				panic(fmt.Sprintf("glog:create file error:%s", err.Error()))
			}
		}
	}
	restoreCounter() //Restore the BufsCounter and fileSizeCounter
}

/**
 * Restore the BufsCounter and fileSizeCounter
 */
func restoreCounter() {
	for fileName := range gruntime.bufsCounter {
		gruntime.bufsCounter[fileName] = 0
		gruntime.fileSizeCounter[fileName] = 0
	}
}

/**
 * get FileName
 */
func getFile(fileName string) (fd *os.File, err error) {
	var suffix, newFileName string //init file suffix

	//make suffix
	switch gruntime.split.id {
	case SPLIT_DAY:
		suffix = time.Now().Format("060102")
	case SPLIT_HOUR:
		suffix = fmt.Sprintf("%s%02d", time.Now().Format("060102"), time.Now().Hour()/gruntime.split.value*gruntime.split.value)
	case SPLIT_MIN:
		suffix = fmt.Sprintf("%s%02d", time.Now().Format("06010215"), time.Now().Minute()/gruntime.split.value*gruntime.split.value)
	case SPLIT_SIZE:

		suffix = "1" // init first logfile suffix

		if gruntime.files[fileName] != nil {
			stat, err := gruntime.files[fileName].Stat()

			if err != nil { //get file status err
				return fd, err
			}

			nameSplit := strings.Split(gruntime.files[fileName].Name(), ".")
			oldSuffix := nameSplit[len(nameSplit)-1]
			if i, err := strconv.Atoi(oldSuffix); stat.Size() > int64(gruntime.split.value*1024*1024) && err == nil {
				i++
				suffix = strconv.Itoa(i)
			} else {
				suffix = oldSuffix
			}
		}
	}

	//make filename
	newFileName = fmt.Sprintf("%s.%s", fileName, suffix)

	//create file
	if gruntime.files[fileName] == nil {
		if fd, err = createFile(newFileName); err == nil {
			gruntime.files[fileName] = fd
		}
	} else if gruntime.files[fileName].Name() != newFileName {
		//close old file, and create new file
		gruntime.files[fileName].Close()
		if fd, err = createFile(newFileName); err == nil {
			gruntime.files[fileName] = fd
		}
	} else {
		fd = gruntime.files[fileName]
	}
	return
}

/**
 * create new disk file
 */
func createFile(fileName string) (fd *os.File, err error) {
	fd, err = os.OpenFile(fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, glog.FileMode)
	return
}

/**
 * parse split chars and return value
 */
func parse_split(split string) (id byte, value int, err error) {
	split = strings.ToLower(split)
	switch split {
	case "day":
		id, value = SPLIT_DAY, 1
	default:
		if strings.HasSuffix(split, "hour") {
			id = SPLIT_HOUR
			value, err = strconv.Atoi(strings.TrimSuffix(split, "hour"))
			//fix illegal hour
			if err == nil && (value < 1 || value > 23) {
				value = 1
			}
		} else if strings.HasSuffix(split, "min") {
			id = SPLIT_MIN
			value, err = strconv.Atoi(strings.TrimSuffix(split, "min"))
			//fix illegal minute
			if err == nil && (value > 60 || value < 5) {
				value = 5
			}
		} else if strings.HasSuffix(split, "m") {
			id = SPLIT_SIZE
			value, err = strconv.Atoi(strings.TrimSuffix(split, "m"))
			//fix illegal size
			if err == nil && value < 1 {
				value = 1
			}
		} else {
			err = errors.New("unknow split")
		}
	}
	return
}
