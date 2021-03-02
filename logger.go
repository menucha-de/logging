package logging

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	//Register sqlite
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

var db *sql.DB
var create string = "CREATE TABLE IF NOT EXISTS record ( millis TIMESTAMP,host TEXT, logger TEXT, method TEXT, level INT, message TEXT, thrown TEXT)"
var insert string = "INSERT INTO record (millis, host,logger,  method, level, message,thrown) VALUES (?, ?, ?, ?, ?,?,?)"
var size string = "SELECT COUNT(*) AS size FROM record WHERE logger like ? and level <= ?"
var selectlog string = "SELECT rowid, strftime('%s', millis) as millis, host, logger,  method, level, message,  thrown FROM record  WHERE logger LIKE ? AND level <= ? ORDER BY millis ASC LIMIT ? OFFSET ?"
var clear string = "DELETE FROM record WHERE logger like ?"
var trunc string = "DELETE FROM record WHERE rowid < ?"
var targets string = "SELECT DISTINCT logger FROM record"
var max int64 = 500
var mapLogger map[string]*Logger = make(map[string]*Logger)
var client mqtt.Client
var brokerURL = "ws://169.254.0.1:9001"
var mapChannels map[string](chan []byte) = make(map[string](chan []byte))
var hostname string
var timeout int = 10000

//Logger custom logger
type Logger struct {
	*logrus.Logger
	off bool
}
type dbHook struct {
	name string
}

var initErr error

//var err error

//Target logging target
type Target struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

//TargetMessage target mqtt message
type TargetMessage struct {
	Target string `json:"target"`
	Host   string `json:"host"`
}

//LogEntry log entry structure
type LogEntry struct {
	ID           int    `json:"id"`
	Time         int64  `json:"time"`
	Host         string `json:"host"`
	TargetName   string `json:"targetName"`
	SourceMethod string `json:"sourceMethod"`
	Level        string `json:"level"`
	Message      string `json:"message"`
	Thrown       string `json:"thrown"`
}

// Message notification message
type Message struct {
	Ref string `json:"ref,omitempty"`
	// PENDING, SUCCESS, FAILURE
	Status string `json:"status,omitempty"`
	Text   string `json:"text,omitempty"`
}

func init() {
	mapChannels["topic"] = make(chan []byte, 100)
	hostname = os.Getenv("LOGHOST")
	db, initErr = sql.Open("sqlite3", "log.db")
	if initErr != nil {
		logrus.Error(initErr)
	} else {
		_, initErr = db.Exec(create)

		if initErr != nil {
			logrus.Error(initErr)
		}
	}

	setBroker()
	go func() {
		if token := client.Connect(); token.WaitTimeout(time.Duration(timeout/1000)*time.Second) && token.Error() == nil {

		} else {
			if token.Error() == nil {
				logrus.Info("Connection timeout")

			} else {
				logrus.Info(token.Error())
			}

		}
		select {
		case <-time.After(60 * time.Second):
			if client != nil && client.IsConnected() && client.IsConnectionOpen() {
			} else {
				if client != nil {
					client.Disconnect(5)
					client = nil
				}
				saveLogstoDatabase()
			}
			return
		}
	}()

}

func connect(server string, id string) mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(server)
	opts.SetConnectTimeout(time.Duration(timeout/1000) * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	//opts.SetCleanSession(false)
	//opts.SetUsername(uri.User.Username())
	//password, _ := uri.User.Password()
	//opts.SetPassword(password)
	//opts.SetConnectionLostHandler(onLost)
	opts.SetOnConnectHandler(onConnect)
	opts.SetClientID(id)
	cl := mqtt.NewClient(opts)

	return cl
}
func onConnect(c mqtt.Client) {
	if c.IsConnectionOpen() {
		logrus.Info("Connection established")

		for topic, m := range mapChannels {
			length := len(m)
			if length > 0 {
				for i := 0; i < length; i++ {
					x := <-m
					c.Publish(topic, 0, false, x)
				}
			}

		}
	}

}
func saveLogstoDatabase() {
	for topic, m := range mapChannels {
		length := len(m)
		if length > 0 {
			for i := 0; i < length; i++ {
				x := <-m
				if strings.HasPrefix(topic, "log/") {
					var e map[string]interface{}
					err := json.Unmarshal(x, &e)
					if err != nil {
						logrus.Error(err)
					}
					savetoDatabase1(e, topic)
				}
			}
		}

	}
}
func (dbHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
		logrus.TraceLevel,
	}
}
func setBroker() {
	id := uuid.New()
	clientid := id.String() //hostname + strconv.Itoa(time.Now().Second())
	if client == nil {
		client = connect(brokerURL, clientid)
	}
}

//SetHost sets the host name for routes shoud be called first
func SetHost(name string) {
	hostname = name
}
func (name dbHook) Fire(e *logrus.Entry) error {
	if strings.TrimSpace(hostname) == "" {
		message := Message{
			Ref:    "Log",
			Status: "FAILURE",
			Text:   fmt.Sprintf("Failed to log for app %s", name.name),
		}
		messageJSON, err := json.Marshal(message)
		if err != nil {
			logrus.Error(err)
			return err
		}
		if client != nil && client.IsConnected() && client.IsConnectionOpen() {
			client.Publish("notification/log", 0, false, messageJSON)
		} else {
			if client != nil {
				select {
				case mapChannels["notification/log"] <- messageJSON:
				case <-time.After(3 * time.Second):
					//client = nil //maybe disconnect
					//it's blocked doing nothing
				}
			}

		}
		return errors.New("message")
	}

	formatter := &logrus.JSONFormatter{}
	formatter.TimestampFormat = "2006-01-02 15:04:05.000"
	e.Data["host"] = hostname
	messageJSON, err := formatter.Format(e)

	if err != nil {
		return err
	}
	//maybe goroutine here
	if client != nil && client.IsConnected() && client.IsConnectionOpen() {
		client.Publish("log/"+name.name, 0, false, messageJSON)

	} else {
		if client != nil {
			select {
			case mapChannels["log/"+name.name] <- messageJSON:
			case <-time.After(3 * time.Second):
				//it's blocked
				savetoDatabase(e, name.name)
			}
		} else {
			savetoDatabase(e, name.name)
		}

	}
	return nil
}
func savetoDatabase(e *logrus.Entry, topic string) {
	if initErr != nil {
		return
	}
	txt := ""
	if e.Data["error"] != nil {
		txt = fmt.Sprintf("%v", e.Data["error"])
	}
	res, err := db.Exec(insert,

		e.Time,
		hostname,
		topic,
		e.Caller.Function+":"+strconv.Itoa(e.Caller.Line),
		e.Level,
		e.Message,
		txt,
	)
	if err != nil {
		logrus.Error(err)
	} else {
		id, err := res.LastInsertId()
		if err != nil {
			logrus.Error(err)
		} else {
			if id > max {
				db.Exec(trunc, id-max+1)
			}
		}
	}
}
func savetoDatabase1(e map[string]interface{}, topic string) {
	if initErr != nil {
		return
	}
	l, err := logrus.ParseLevel(strings.ToUpper(fmt.Sprintf("%v", e["level"])))
	if err != nil {
		l = 6
	}
	res, err := db.Exec(insert,

		e["time"],
		hostname,
		topic,
		e["file"],
		l,
		e["msg"],
		e["error"],
	)
	if err != nil {
		logrus.Error(err)
	} else {
		id, err := res.LastInsertId()
		if err != nil {
			logrus.Error(err)
		} else {
			if id > max {
				db.Exec(trunc, id-max+1)
			}
		}
	}
}

//Notify used for notification only
func Notify(topic string, message Message) {
	messageJSON, err := json.Marshal(message)
	if err != nil {
		logrus.Error(err)
		return
	}
	if client != nil && client.IsConnected() && client.IsConnectionOpen() {
		client.Publish("notification/"+topic, 0, false, messageJSON)
		return
	}
	_, ok := mapChannels["notification/"+topic]
	if !ok {
		mapChannels["notification/"+topic] = make(chan []byte, 100)
	}
	if client != nil {
		select {
		case mapChannels["notification/"+topic] <- messageJSON:
		case <-time.After(3 * time.Second):
			//client = nil //maybe disconnect
			//..it's blocked doing nothing
		}
	}

}
func newLogger(name string) *Logger {
	l := logrus.New()

	l.SetReportCaller(true)
	if initErr == nil {

		l.AddHook(dbHook{name})
	}
	return &Logger{
		Logger: l,
	}
}

//GetLogger returns an instance of logger
func GetLogger(name string) *Logger {

	l, ok := mapLogger[name]
	if !ok {
		mapChannels["log/"+name] = make(chan []byte, 100)
		mapLogger[name] = newLogger(name)
		//mapLogger[name].Info("initialized") //force log once
		publishlogger(name)
		return mapLogger[name]
	}
	return l
}
func publishlogger(name string) {
	if strings.TrimSpace(hostname) == "" && name != "art" {
		message := Message{
			Ref:    "Log",
			Status: "FAILURE",
			Text:   fmt.Sprintf("Failed to initialize logging for %s", name),
		}
		messageJSON, err := json.Marshal(message)
		if err != nil {
			logrus.Error(err)
			return
		}
		if client != nil && client.IsConnected() && client.IsConnectionOpen() {
			client.Publish("notification/log", 0, false, messageJSON)
		} else {
			if client != nil {
				select {
				case mapChannels["notification/log"] <- messageJSON:
				case <-time.After(3 * time.Second):
					//client = nil //maybe disconnect
					//it's blocked doing nothing
				}
			}

		}
		return
	}

	a := TargetMessage{name, hostname}
	message, err := json.Marshal(a)
	if err != nil {
		logrus.Error(err)
		return
	}
	if client != nil && client.IsConnected() && client.IsConnectionOpen() {
		client.Publish("topic", 0, false, message)

	} else {
		if client != nil {
			select {
			case mapChannels["topic"] <- message:
			case <-time.After(3 * time.Second):
				//client = nil //maybe disconnect
				//it's blocked doing nothing
			}
		}

	}

}

//Close close the database connection
func Close() {
	db.Close()
	if client != nil {
		client.Disconnect(10)
		client = nil
	}
}

func getTargets() []Target {

	var s []Target
	s = make([]Target, 0)
	for v := range mapLogger {

		b := Target{
			Name:  v,
			Label: v,
		}
		s = append(s, b)
	}
	return s
}

func getSize(name string, level int) int {

	if client != nil && client.IsConnected() && client.IsConnectionOpen() {
		return 1
	}

	var count int
	if name == "ALL" {
		name = "%"
	}

	err := db.QueryRow(size, name, level).Scan(&count)
	switch {
	case err == sql.ErrNoRows:
		return 0
	case err != nil:
		logrus.Error("query error:" + err.Error())
		return 0
	default:
		return count
	}

}

func getLogs(logger string, level int, limit int, offset int) []LogEntry {

	if client != nil && client.IsConnected() && client.IsConnectionOpen() {
		s := make([]LogEntry, 0)
		b := LogEntry{
			ID:           0,
			Time:         time.Now().Unix() * 1000,
			TargetName:   logger,
			SourceMethod: "",
			Level:        "INFO",
			Message:      "Logging is available in the central activity log",
			Thrown:       "",
		}
		s = append(s, b)
		return s
	}

	if logger == "ALL" {
		logger = "%"
	}

	rows, err := db.Query(selectlog, logger, level, limit, offset)

	if err != nil {
		logrus.Error(err)
		return nil
	}
	defer rows.Close()
	var s []LogEntry
	s = make([]LogEntry, 0)
	var (
		mlogger, mmethod, message, thrown string
		mlevel, id                        int
		millis                            int64
	)
	for rows.Next() {

		rows.Scan(&id, &millis, &mlogger, &mmethod, &mlevel, &message, &thrown)
		i := logrus.Level(mlevel)
		var lv string
		if b, err := i.MarshalText(); err == nil {
			lv = strings.ToUpper(string(b))
		} else {
			lv = "unknown"
		}
		b := LogEntry{
			ID:           id,
			Time:         millis * 1000,
			TargetName:   mlogger,
			SourceMethod: mmethod,
			Level:        lv,
			Message:      message,
			Thrown:       thrown,
		}

		s = append(s, b)
	}
	return s
}

func deleteLogs(target string) {

	if target == "ALL" {
		target = "%"
	}

	_, err := db.Exec(clear, target)
	if err != nil {
		logrus.Error(err.Error())
	}

}

func getLogLvl(target string) string {

	l := GetLogger(target)
	if l.off {
		return "OFF"
	}

	lev := l.GetLevel()
	x, _ := lev.MarshalText()
	xx := string(bytes.ToUpper(x))
	return xx
}
