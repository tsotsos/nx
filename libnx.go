package main

import (
	"crypto/tls"
	"encoding/xml"
	"errors"
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
}

// Stores all statuses for a Zone
type ZoneStatus struct {
	Ready        bool
	ByPass       bool
	SysCondition bool
	InAlarm      bool
}

// Stores latest data for Zones such as
// names, statuses and total number
type Zones struct {
	Names  []string
	Status []ZoneStatus
	Number int `len(Status)`
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

type httpRequest struct {
	Path   string
	Method string
	Params string
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

// session id global
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

// ZoneStatuses fetch status for each zone in the system
func ZonesStatuses(conf *Config) ([]ZoneStatus, error) {
	rawSequence, err := Sequence(conf)
	zones := strings.Split(rawSequence.Zones, ",")
	zonesData := make([][4]int, len(zones))
	for i, _ := range zones {
		zstate, _ := Zstate(conf, i)
		zonesData[zstate.Zstate] = zstate.Zdat
	}
	zonesStatus := calculateStatuses(zonesData)
	return zonesStatus, err
}

// Creates an array of statuses for zones
func calculateStatuses(zones [][4]int) []ZoneStatus {
	result := make([]ZoneStatus, len(zones))
	for i, _ := range zones {
		result[i] = calculateStatus(i, zones)
	}
	return result
}

// Calculates status for a zone
func calculateStatus(i int, zones [][4]int) ZoneStatus {
	mask := 0x01 << (uint(i) % 16)
	byteIndex := int(math.Floor(float64(i) / 16))
	status := ZoneStatus{false, false, false, false}
	// In alarm
	if zones[5][byteIndex]&mask != 0 {
		status.InAlarm = true
	}
	// System condition
	if zones[1][byteIndex]&mask != 0 || zones[2][byteIndex]&mask != 0 ||
		zones[6][byteIndex]&mask != 0 || zones[7][byteIndex]&mask != 0 {
		status.SysCondition = true
	}
	// ByPass
	if zones[3][byteIndex]&mask != 0 || zones[4][byteIndex]&mask != 0 {
		status.ByPass = true
	}
	// Not Ready
	if zones[0][byteIndex]&mask == 0 {
		status.Ready = true
	}
	return status
}

// Retrieves zone names via parsing embeded javascript variable from
// zones.htm file. Unfortunately no other way found
func ZonesNames(conf *Config) ([]string, error) {
	var data httpRequest
	var names []string
	data.Path = conf.Url + "user/zones.htm"
	data.Params = ""
	data.Method = "GET"
	response, err := doRequest(data, conf, 2)
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

func addSession(params string, session string) string {
	if session != "" {
		return "sess=" + session + "&" + params
	}
	return params
}

// HTTP request wrapper. Responsible for all requests. Accept httpRequest struct
// and Config. Also handles re-try, in case of expired session it may re-login
// if tries is greater than 1
func doRequest(data httpRequest, conf *Config, tries int) ([]byte, error) {
	var result []byte
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	body := strings.NewReader(data.Params)
	request, err := http.NewRequest(data.Method, data.Path, body)
	if err != nil {
		return result, err
	}
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	bodyBytes, errBody := ioutil.ReadAll(response.Body)
	if errBody != nil {
		return result, errBody
	}
	// In case of session expire and given enought tries we handle re-login
	if response.StatusCode == http.StatusForbidden ||
		(response.StatusCode == http.StatusOK &&
			loginFormExist(bodyBytes) == true) {
		if tries > 1 {
			if data.Path == "login.cgi" == false {
				newSession, _ := login(conf)
				data.Params = addSession(data.Params, newSession)
			}
			return doRequest(data, conf, tries-1)
		}
		return result, err
	}
	if response.StatusCode != http.StatusOK {
		return result, errors.New("Could not connect to card")
	}
	defer response.Body.Close()
	return bodyBytes, err
}

// Login to system this function returns session id and also save it to a file
// and global
func login(conf *Config) (string, error) {
	var result string
	var session string
	var data httpRequest
	re := regexp.MustCompile(
		`(?msUi)function getSession\(\){return\s"(\S.*)";}`)

	data.Path = conf.Url + "login.cgi"
	data.Params = "lgname=" + conf.User + "&" + "lgpin=" + conf.Pin
	data.Method = "POST"
	response, err := doRequest(data, conf, 1)

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

// Status fetches System Statusfrom HTTP server and handles reconnection
// in case session has been expired
func Status(conf *Config) (SystemStatus, error) {
	var data httpRequest
	data.Path = conf.Url + "user/status.xml"
	data.Params = addSession("", getSession())
	data.Method = "POST"
	response, err := doRequest(data, conf, 2)
	result := SystemStatus{}
	if err != nil {
		return result, err
	}
	xml.Unmarshal(response, &result)
	return result, err
}

// Returns Sequence. Via this request we can retrieve seq.xml response but still
// in order to retrieve Statuses you should use ZoneStatuses function, since
// sequence cannot be used without Zstate.
func Sequence(conf *Config) (sequenceReq, error) {
	var data httpRequest
	data.Path = conf.Url + "user/seq.xml"
	data.Params = addSession("", getSession())
	data.Method = "POST"
	response, err := doRequest(data, conf, 2)
	result := sequenceReq{}
	if err != nil {
		return result, err
	}
	xml.Unmarshal(response, &result)
	return result, err
}

// Returns Zstate result. Zstate cannot be used without Sequence, only by
// joining Zstate and Sequese requests we can calculate zone statues
// for this use ZoneStatuses()
func Zstate(conf *Config, state int) (zstateReq, error) {
	var data httpRequest
	data.Path = conf.Url + "user/zstate.xml"
	result := zstateReq{}
	data.Method = "POST"
	data.Params = addSession("state="+strconv.Itoa(state), getSession())
	response, err := doRequest(data, conf, 2)
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

// Sets a zone to "Bypass state
func SetByPass(zone int, conf *Config) error {
	var data httpRequest
	params := "comm=82&data0=" + strconv.Itoa(zone)
	data.Path = conf.Url + "user/zonefunction.cgi"
	data.Params = addSession(params, getSession())
	data.Method = "POST"
	_, err := doRequest(data, conf, 2)
	return err
}
