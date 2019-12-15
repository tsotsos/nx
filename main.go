package main

import (
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

type SystemStatus struct {
	Abank          int    `xml:"abank"`
	Seq            int    `xml:"aseq"`
	Away           bool   `xml:"stat0"`
	Stay           bool   `xml:"stat1"`
	Ready          bool   `xml:"stat2"`
	FireAlarm      bool   `xml:"stat3"`
	IntrusionAlarm bool   `xml:"stat4"`
	ExitDelay      bool   `xml:"stat7"`
	EntryDelay     bool   `xml:"stat9"`
	BypassOn       bool   `xml:"stat10"`
	ChimeOn        bool   `xml:"stat15"`
	Message        string `xml:"sysflt"`
}

var sessionId string

func init() {
	sessionId = ""
	if err := godotenv.Load(); err != nil {
		fmt.Println(err)
	}
}

func main() {
	conf := NewConfig()
	fmt.Println(conf)
	st, err := Status(conf)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(st)
}

// Status fetches System Statusfrom HTTP server and handles reconnection
// in case session has been expired
func Status(conf *Config) (SystemStatus, error) {
	status := SystemStatus{}
	// we try current setted session
	status, err := getStatus(conf)
	if err.Error() == "Forbidden" {
		sessionId, err = login(conf)
		if err != nil {
			return status, err
		}
		status, err = getStatus(conf)
	}
	return status, err
}

func doRequest(url string, method string, params string) ([]byte, error) {

	var result []byte
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	//if method == "POST" {
	body := strings.NewReader(params)
	request, err := http.NewRequest(method, url, body)
	//}
	if err != nil {
		return result, err
	}
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	if response.StatusCode == http.StatusForbidden {
		return result, errors.New("Forbidden")
	}
	if response.StatusCode != http.StatusOK {
		return result, errors.New("Could not connect to card")
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	return bodyBytes, err
}

func login(conf *Config) (string, error) {
	var result string
	var session string
	re := regexp.MustCompile(
		`(?msUi)function getSession\(\){return\s"(\S.*)";}`)
	params := "lgname=" + conf.User + "&lgpin=" + conf.Pin
	url := conf.Url + "login.cgi"
	response, err := doRequest(url, "POST", params)

	if err != nil {
		return result, err
	}
	bodyString := string(response)
	for _, match := range re.FindAllStringSubmatch(bodyString, -1) {
		if match[1] != "" {
			session = match[1]
		}
	}
	return session, err
}

func getStatus(conf *Config) (SystemStatus, error) {
	params := "sess=" + sessionId + "&arsel=0"
	url := conf.Url + "user/status.xml"
	response, err := doRequest(url, "POST", params)
	result := SystemStatus{}
	if err != nil {
		return result, err
	}
	xml.Unmarshal(response, &result)
	return result, err
}
