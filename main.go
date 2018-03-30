package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

var CONFIG map[string]map[string]string

const COMMENT_FLAG = "#"

var RE_SESSION *regexp.Regexp = regexp.MustCompile("^\\[.+\\]$")
var RE_KV *regexp.Regexp = regexp.MustCompile("^.+\\=.+$")
var RC_FILES []string
var AUTORELOAD int32

var mutex sync.Mutex

func exists(file string) bool {
	if f, err := os.Open(file); err != nil && os.IsNotExist(err) {
		defer f.Close()
		return false

	}
	return true
}

func parse_line(line string) (string, interface{}) {
	comment_index := strings.Index(line, COMMENT_FLAG)
	if comment_index != -1 {
		line = line[0:comment_index]
	}
	if strings.TrimSpace(line) == "" {
		return "empty_line", nil
	}
	if RE_SESSION.MatchString(line) {
		return "section", line[1 : len(line)-1]
	}

	if RE_KV.MatchString(line) {
		parts := strings.SplitN(line, "=", 2)
		return "kv", map[string]string{strings.TrimSpace(parts[0]): strings.TrimSpace(parts[1])}
	}

	fmt.Fprintf(os.Stderr, "%s format error!\n", line)
	return "other", nil
}

type Config map[string]string

func (c Config) GetInt64(k string) int64 {
	v := c[k]

	i, err := strconv.ParseInt(v, 10, 0)
	if err != nil {
		panic(errors.Wrap(err, "ParseInt"))
	}

	return i
}

func (c Config) GetInt(k string) int {
	return int(c.GetInt64(k))
}

func (c Config) GetFloat64(k string) float64 {
	v := c[k]

	f, err := strconv.ParseFloat(v, 0)
	if err != nil {
		panic(errors.Wrap(err, "ParseFloat"))
	}

	return f
}

func Get(section string) (Config, error) {
	mutex.Lock()
	defer mutex.Unlock()
	_section_config := make(Config)

	config, ok := CONFIG[section]
	if !ok {
		return nil, fmt.Errorf("section %s not found", section)
	}
	for name, value := range config {
		_section_config[name] = value
	}
	return _section_config, nil
}

func MustGet(section string) Config {
	config, err := Get(section)
	if err != nil {
		panic(err)
	}

	return config
}

func Sections() (sections []string) {
	for section, _ := range CONFIG {
		sections = append(sections, section)
	}
	return sections
}

func parse_rc_content(content string) map[string]map[string]string {
	config := make(map[string]map[string]string)
	lines := strings.FieldsFunc(content, func(char rune) bool {
		return strings.ContainsRune("\r\n", char)
	})

	last_section := ""

	for _, line := range lines {
		line_type, line_value := parse_line(line)
		switch {
		case line_type == "empty_line":
			continue

		case line_type == "section":
			section := line_value.(string)
			if _, ok := config[section]; ok == false {
				config[section] = make(map[string]string)
			}
			last_section = section

		case line_type == "kv" && last_section != "":
			kv := line_value.(map[string]string)
			for k, v := range kv {
				config[last_section][k] = v
			}
		}
	}

	return config
}

func InitReader(in io.Reader) {
	content_bytes, err := ioutil.ReadAll(in)
	if err != nil {
		log.Fatalf("%+v\n", errors.Wrap(err, "ReadAll"))
	}

	content := string(content_bytes)
	for section, configs := range parse_rc_content(content) {
		_, ok := CONFIG[section]
		if !ok {
			CONFIG[section] = make(map[string]string)
		}

		for name, value := range configs {
			CONFIG[section][name] = value
		}
	}
}

func Init(rc_files ...string) {
	mutex.Lock()
	defer mutex.Unlock()
	RC_FILES = rc_files

	for _, rc_file := range rc_files {
		if !exists(rc_file) {
			continue
		}
		CONFIG = make(map[string]map[string]string)
		f, err := os.Open(rc_file)
		if err != nil {
			log.Fatalf("%+v\n", errors.Wrap(err, "os.Open"))
		}
		InitReader(f)
	}
}

func ReInit() error {
	if RC_FILES == nil || len(RC_FILES) == 0 {
		return errors.New("not init")
	}
	Init(RC_FILES...)
	return nil
}

func reload(callback func(section, k, v string)) {
	// map copy
	old_config := make(map[string]map[string]string)
	b, _ := json.Marshal(CONFIG)
	json.Unmarshal(b, &old_config)

	ReInit() // reload

	if callback == nil {
		return
	}

	for section, configs := range CONFIG {
		old_configs, _ := old_config[section]
		if old_configs == nil { // 新增的section不需要匹配
			continue
		}
		for k, v := range configs {
			old_v, _ := old_configs[k]
			if v != old_v {
				callback(section, k, v)
			}
		}
	}

}

func OpenAutoReLoad(callback func(section, k, v string)) {
	if !atomic.CompareAndSwapInt32(&AUTORELOAD, 0, 1) {
		panic("already open autoload")
	}
	go func() {
		rc_update_ts := make(map[string]int64)
		for {
			for _, rc_file := range RC_FILES {
				info, err := os.Stat(rc_file)
				if err != nil {
					continue
				}
				if last_update_ts, ok := rc_update_ts[rc_file]; ok {
					if last_update_ts < info.ModTime().Unix() {
						reload(callback)
					}

				} else {
					rc_update_ts[rc_file] = info.ModTime().Unix()
				}

			}
			time.Sleep(1 * time.Second)
		}
	}()

	return
}

func CloseAutoLoad() {
	atomic.CompareAndSwapInt32(&AUTORELOAD, 1, 0)
}
