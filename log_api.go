package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/peramic/utils"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func getLogLevels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	var levels [8]string
	var i int
	levels[0] = "OFF"
	i++
	for _, v := range logrus.AllLevels {
		x, _ := v.MarshalText()
		xx := string(bytes.ToUpper(x))
		levels[i] = xx
		i++
	}
	//levels[i] = "ALL"
	enc := json.NewEncoder(w)
	err := enc.Encode(levels)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getLogTargets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	targets := getTargets()
	enc := json.NewEncoder(w)
	err := enc.Encode(targets)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

const (
	// error messages
	notAvailable = "Log level \"%s\" is not available"
)

func getLogSize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	vars := mux.Vars(r)
	level := vars["level"]
	var lv int
	if level == "ALL" {
		lv = 10
	} else {
		lvv, err := logrus.ParseLevel(strings.ToLower(level))
		if err != nil {
			errMsg := fmt.Sprintf(notAvailable, level)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
		lv = int(lvv)
	}
	target := vars["target"]

	size := getSize(target, lv)
	enc := json.NewEncoder(w)
	err := enc.Encode(size)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getLogEntries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	vars := mux.Vars(r)
	target := vars["target"]
	level := vars["level"]
	var lv int
	if level == "ALL" {
		lv = 10
	} else {
		lvv, err := logrus.ParseLevel(strings.ToLower(level))
		if err != nil {
			errMsg := fmt.Sprintf(notAvailable, level)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
		lv = int(lvv)
	}
	limit := vars["limit"]
	offset := vars["offset"]
	lim, _ := strconv.Atoi(limit)
	offs, _ := strconv.Atoi(offset)
	res := getLogs(target, lv, lim, offs)
	enc := json.NewEncoder(w)
	err := enc.Encode(res)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func deleteLogEntries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	vars := mux.Vars(r)
	target := vars["target"]
	deleteLogs(target)
	w.WriteHeader(http.StatusNoContent)
}

func getLogLevel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	vars := mux.Vars(r)
	target := vars["target"]
	level := getLogLvl(target)

	enc := json.NewEncoder(w)
	err := enc.Encode(level)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func setLogLevel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	target := vars["target"]
	var level string
	err := utils.DecodeJSONBody(w, r, &level)
	if err != nil {
		var mr *utils.MalformedRequest
		if errors.As(err, &mr) {
			http.Error(w, mr.Msg, mr.Status)
		} else {
			logrus.Error(err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	l := GetLogger(target)
	if level == "OFF" {
		l.Out = ioutil.Discard
		l.off = true
	} else {
		lv, err := logrus.ParseLevel(strings.ToLower(level))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		l.off = false
		l.SetLevel(lv)
	}
	w.WriteHeader(http.StatusNoContent)
}

func getLogFile(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	target := vars["target"]
	level := vars["level"]
	t := time.Now()
	formatted := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())
	fileName := fmt.Sprintf("Log_%s_%s_%s.txt", target, level, formatted)
	var lv int
	if level == "ALL" {
		lv = 10
	} else {
		lvv, err := logrus.ParseLevel(strings.ToLower(level))
		if err != nil {
			errMsg := fmt.Sprintf(notAvailable, level)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
		lv = int(lvv)
	}
	res := getLogs(target, lv, -1, 0)
	data := []byte("")
	for _, v := range res {
		s, err := json.Marshal(v)
		if err == nil {
			data = append(data, s...)
			data = append(data, []byte{10}...)
		}
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
	return
}
