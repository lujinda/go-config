package config

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"testing"
	"time"
)

func updateName(name string) {
	b, err := ioutil.ReadFile("t1.cfg")
	if err != nil {
		panic(err)
	}
	re := regexp.MustCompile("name = \\w+")
	content := re.ReplaceAllString(string(b), fmt.Sprintf("name = %s", name))
	err = ioutil.WriteFile("t1.cfg", []byte(content), 077)
	if err != nil {
		panic(err)
	}
}

func TestConfig(t *testing.T) {
	updateName("ljd")
	Init("t1.cfg")
	if len(Sections()) == 0 {
		t.Error(Sections())
	}

	c := MustGet("section1")
	if c.GetFloat64("height") != 180.1 {
		t.Fatal("my height?")
	}

	if c.GetInt("age") != 20 {
		t.Fatal("my age?")
	}

	var randstring string
	randstring = fmt.Sprintf("%x", md5.Sum([]byte(strconv.FormatInt(time.Now().UnixNano(), 10))))[0:5]
	if MustGet("section1")["name"] != "ljd" {
		t.Fatalf("name is %s, not %s\n", MustGet("section1")["name"], "ljd")
	}

	OpenAutoReLoad(func(section, k, v string) {
		fmt.Println("new", section, k, v)
		if v != randstring {
			t.Fatalf("name is %s, not %s\n", v, randstring)
		}
	})

	time.Sleep(1 * time.Second)

	updateName(randstring)
	time.Sleep(2 * time.Second)

	if MustGet("section1")["name"] != randstring {
		t.Fatalf("name is %s, not %s\n", MustGet("section1")["name"], randstring)

	}

	randstring = fmt.Sprintf("%x", md5.Sum([]byte(strconv.FormatInt(time.Now().UnixNano(), 10))))[0:5]
	updateName(randstring)
	time.Sleep(2 * time.Second)
	if MustGet("section1")["name"] != randstring {
		t.Fatalf("name is %s, not %s\n", MustGet("section1")["name"], randstring)
	}

}
