package main

import (
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	Protocol string
	Host     string
	Name     string
	User     string
	Pin      string
	Url      string
	Session  string //session id
}

const (
	Ready = iota
	NotReady
	ByPass
	SysCondition
	InAlarm
)

// Stores latest data for Zones such as
// names, statuses and total number
type Zones struct {
	Number int
	Names  []string
	Status []int
}

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

type sequenceReq struct {
	Areas int    `xml:"areas"`
	Zones string `xml:"zones"`
}

type zstateReq struct {
	Zstate int    `xml:"zstate"`
	Zseq   int    `xml:"zseq"`
	ZdatS  string `xml:"zdat"`
	Zdat   [4]int
}

var sessionId string

// Sets session to global and file
func setSession(session string) {
	sessionId = session
	file, err := os.Create("session")
	if err != nil {
		panic(err)
		return
	}
	defer file.Close()
	file.WriteString(session)
}

// Retrieves the session from global or file
func getSession() string {
	if sessionId != "" {
		return sessionId
	}
	content, err := ioutil.ReadFile("session")
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		panic(err)
	}
	session := string(content)
	sessionId = session
	return session
}

func ZoneNames(conf *Config) {
	data, err := zonesRq(conf)
	fmt.Println(data)
	if err != nil {
		fmt.Println(err)
	}
}

// ZoneStatuses fetch status for each zone in the system
func ZonesStatuses(conf *Config) (string, error) {
	var rawSequence sequenceReq
	rawSequence, err := Sequence(conf)
	zones := strings.Split(rawSequence.Zones, ",")
	zonesData := make([][4]int, len(zones))
	for i, _ := range zones {
		//		n, _ := strconv.Atoi(zones[i])
		//		if n != zonesData[i] {
		zstate, _ := Zstate(conf, i)
		//if err != nil {
		//	panic(err)
		//}
		zonesData[zstate.Zstate] = zstate.Zdat
		//spew.Dump(zstate)
		//		}
	}
	fmt.Println(zonesZStatus(zonesData))
	return "", err
}

// Returns Sequence. Via this request we can retrieve seq.xml response but still
// in order to retrieve Statuses you should use ZoneStatuses function, since
// sequence cannot be used without Zstate.
func Sequence(conf *Config) (sequenceReq, error) {
	// we try current setted session
	result := sequenceReq{}
	result, err := getSequence(conf)
	if err != nil && err.Error() == "Forbidden" {
		_, err := login(conf)
		if err != nil {
			return result, err
		}
		result, err = getSequence(conf)
	}
	return result, err
}

// Returns Zstate result. Zstate cannot be used without Sequence, only by
// joining Zstate and Sequese requests we can calculate zone statues
// for this use ZoneStatuses()
func Zstate(conf *Config, state int) (zstateReq, error) {
	// we try current setted session
	result := zstateReq{}
	result, err := getZstate(conf, state)
	if err != nil && err.Error() == "Forbidden" {
		_, err := login(conf)
		if err != nil {
			return result, err
		}
		result, err = getZstate(conf, state)
	}
	return result, err
}

// Status fetches System Statusfrom HTTP server and handles reconnection
// in case session has been expired
func Status(conf *Config) (SystemStatus, error) {
	status := SystemStatus{}
	// we try current setted session
	status, err := getStatus(conf)
	if err != nil && err.Error() == "Forbidden" {
		_, err := login(conf)
		if err != nil {
			return status, err
		}
		status, err = getStatus(conf)
	}
	return status, err
}

// Handles the case of login form prompt. In case of session expiration in some
// occasions NX redirects to login page (even on XHRs), this function dectects
// login form and returns its status
func loginFormExist(response []byte) bool {
	loginForm := ""
	var re = regexp.MustCompile(`(?m)form method="post" action="/login.cgi"`)
	for _, match := range re.FindAllStringSubmatch(string(response), -1) {
		if match[0] != "" {
			loginForm = match[0]
		}
	}
	if loginForm != "" {
		return true
	}
	return false
}

// Creates an array of statuses for zones
func zonesZStatus(zones [][4]int) []int {
	result := make([]int, len(zones))
	for i, _ := range zones {
		result[i] = zoneZStatus(i, zones)
	}
	return result
}

// Calculates status for a zone
func zoneZStatus(i int, zones [][4]int) int {
	mask := 0x01 << (uint(i) % 16)
	byteIndex := int(math.Floor(float64(i) / 16))

	// In alarm
	if zones[5][byteIndex]&mask != 0 {
		return InAlarm
	}
	// System condition
	if zones[1][byteIndex]&mask != 0 || zones[2][byteIndex]&mask != 0 ||
		zones[6][byteIndex]&mask != 0 || zones[7][byteIndex]&mask != 0 {
		return SysCondition
	}
	// ByPass
	if zones[3][byteIndex]&mask != 0 || zones[4][byteIndex]&mask != 0 {
		return ByPass
	}
	// Not Ready
	if zones[0][byteIndex]&mask != 0 {
		return NotReady
	}
	return Ready

}

// HTTP request wrapper
func doRequest(path string, method string, params url.Values) ([]byte, error) {
	var result []byte
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	body := strings.NewReader(params.Encode())
	request, err := http.NewRequest(method, path, body)
	if err != nil {
		return result, err
	}
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	// In case of session expire returns an error "Forbidden" so we can
	// handle re-login
	if response.StatusCode == http.StatusForbidden {
		return result, errors.New("Forbidden")
	}
	if response.StatusCode != http.StatusOK {
		fmt.Println(response.StatusCode)
		fmt.Println(response.Header)
		return result, errors.New("Could not connect to card")
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	// same as above returns forbidden in case a login form exists.
	if loginFormExist(bodyBytes) == true {
		return result, errors.New("Forbidden")
	}
	defer response.Body.Close()
	return bodyBytes, err
}

// Login to system this function returns session id and also save it to a file
// and global
func login(conf *Config) (string, error) {
	var result string
	var session string
	path := conf.Url + "login.cgi"
	re := regexp.MustCompile(
		`(?msUi)function getSession\(\){return\s"(\S.*)";}`)
	params := url.Values{}
	params.Add("lgname", conf.User)
	params.Add("lgpin", conf.Pin)
	response, err := doRequest(path, "POST", params)

	if err != nil {
		return result, err
	}
	bodyString := string(response)
	for _, match := range re.FindAllStringSubmatch(bodyString, -1) {
		if match[1] != "" {
			session = match[1]
		}
	}
	setSession(session)
	return session, err
}

// Handles the status request and response parsing
func getStatus(conf *Config) (SystemStatus, error) {
	path := conf.Url + "user/status.xml"
	params := url.Values{}
	params.Add("sess", getSession())
	params.Add("arsel", "7")
	response, err := doRequest(path, "POST", params)
	fmt.Println(string(response))
	result := SystemStatus{}
	if err != nil {
		return result, err
	}
	xml.Unmarshal(response, &result)
	return result, err
}

// Handles Sequence request
func getSequence(conf *Config) (sequenceReq, error) {
	params := url.Values{}
	params.Add("sess", getSession())
	path := conf.Url + "user/seq.xml"
	response, err := doRequest(path, "POST", params)
	result := sequenceReq{}
	if err != nil {
		return result, err
	}
	xml.Unmarshal(response, &result)
	return result, err
}

// Handles Zstate request
func getZstate(conf *Config, state int) (zstateReq, error) {
	path := conf.Url + "user/zstate.xml"
	result := zstateReq{}
	params := url.Values{}
	params.Add("sess", getSession())
	params.Add("state", strconv.Itoa(state))
	response, err := doRequest(path, "POST", params)
	xml.Unmarshal(response, &result)
	if result.ZdatS != "" {
		stAr := strings.Split(result.ZdatS, ",")
		for i, v := range stAr {
			result.Zdat[i], err = strconv.Atoi(v)
			if err != nil {
				return result, err
			}
		}
	}
	return result, err
}

// Handles request for zone names and parsing embeded javascript variable
// from zones.html file.
// Unfortunately no other way found
func zonesRq(conf *Config) ([]string, error) {
	var names []string
	path := conf.Url + "user/zones.htm"
	params := url.Values{}
	params.Add("sess", getSession())
	response, err := doRequest(path, "GET", params)
	if err != nil {
		return names, err
	}
	var re = regexp.MustCompile(`(?m)var zoneNames = new\sArray\((.*)\);`)
	for _, match := range re.FindAllStringSubmatch(string(response), -1) {
		if match[1] == "" {
			continue
		}
		sb := strings.Split(match[1], ",")
		if len(sb) == 0 {
			continue
		}
		names = make([]string, len(sb))
		for i, z := range sb {
			names[i], err = url.PathUnescape(z)
			if err != nil {
				names[i] = z
			}
		}
	}
	return names, err
}
