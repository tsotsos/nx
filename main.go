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

func init() {
	if err := godotenv.Load(); err != nil {
		fmt.Println(err)
	}
}
func main() {
	Conf := NewConfig()
	//session, err := LoginRq()
	//session, err := Login("", "")
	//	if err != nil {
	//		fmt.Println(err)
	//	}
	//	st, _ := Status("CC653D4AAAA947FF")
	//	if err != nil {
	//		panic(err)
	//	}
	fmt.Println("")
}

// Status fetches System Statusfrom HTTP server and handles reconnection
// in case session has been expired
//func Status() (error, SystemStatus) {
//}

func doRequest(call string, method string, params string) ([]byte, error) {

	var result []byte
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	url := Conf.Protocol + "://" + Conf.Ip + "/" + call

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

func login(user string, password string) (string, error) {
	var result string
	var session string
	re := regexp.MustCompile(
		`(?msUi)function getSession\(\){return\s"(\S.*)";}`)
	params := "lgname=" + user + "&lgpin=" + password
	response, err := doRequest("login.cgi", "POST", params)

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

func getStatus(session string) (SystemStatus, error) {
	params := "sess=" + session + "&arsel=0"
	response, err := doRequest("user/status.xml", "POST", params)
	result := SystemStatus{}
	if err != nil {
		return result, err
	}
	xml.Unmarshal(response, &result)
	return result, err
}
