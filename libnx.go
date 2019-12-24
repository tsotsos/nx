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

type Settings struct {
	Protocol string
	Host     string
	Name     string
	User     string
	Pin      string
	Url      string
}

// All system triggers
const (
	Arm = iota
	Stay
	Disarm
	Chime
)

type NxAlarm struct {
	System   systemStatus
	Zones    zones
	Settings Settings
}

// Complete information about system and zones status
// TODO: Should re-implement this structure to support multiple areas
type systemStatus struct {
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

// Stores all statuses for a Zone
type zoneStatus struct {
	Ready        bool
	ByPass       bool
	SysCondition bool
	InAlarm      bool
}

// Stores latest data for Zones such as
// names, statuses and total number
type zones struct {
	Names  []string
	Status []zoneStatus
}

// Keeps all data needed for a NX card HTTP request
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

// Settings creation
func NewSettings() *Settings {
	return &Settings{
		Protocol: getEnv("NX_PROTOCOL", ""),
		Host:     getEnv("NX_HOST", ""),
		Name:     getEnv("NX_NANE", ""),
		User:     getEnv("NX_USER", ""),
		Pin:      getEnv("NX_PIN", ""),
		Url: getEnv("NX_PROTOCOL", "") + "://" +
			getEnv("NX_HOST", "") + "/",
	}
}

func NewNxAlarm() *NxAlarm {
	return &NxAlarm{
		System: systemStatus{},
		Zones:  zones{},
		Settings: Settings{
			Protocol: getEnv("NX_PROTOCOL", ""),
			Host:     getEnv("NX_HOST", ""),
			Name:     getEnv("NX_NANE", ""),
			User:     getEnv("NX_USER", ""),
			Pin:      getEnv("NX_PIN", ""),
			Url: getEnv("NX_PROTOCOL", "") + "://" +
				getEnv("NX_HOST", "") + "/",
		},
	}
}

// Status fetches System Statusfrom HTTP server
func (nx *NxAlarm) SystemStatus() (*NxAlarm, error) {
	var data httpRequest
	var result systemStatus
	data.Path = nx.Settings.Url + "user/status.xml"
	data.Params = addSession("", getSession())
	data.Method = "POST"
	response, err := makeRequest(data, nx.Settings, 2)
	if err != nil {
		return nx, err
	}
	xml.Unmarshal(response, &result)
	nx.System = result
	return nx, err
}

// ZonesStatus fetches zone names and their statuses
func (nx *NxAlarm) ZonesStatus() (*NxAlarm, error) {
	// retrieves stored zone names
	names, err := zonesNames(nx.Settings)
	if err != nil {
		return nx, err
	}
	// retrieves and caclulate various zone
	// statuses
	rawSequence, err := sequence(nx.Settings)
	zones := strings.Split(rawSequence.Zones, ",")
	zonesData := make([][4]int, len(zones))
	for i, _ := range zones {
		zstate, _ := zstate(nx.Settings, i)
		zonesData[zstate.Zstate] = zstate.Zdat
	}
	nx.Zones.Status = calculateStatuses(zonesData)
	nx.Zones.Names = names
	return nx, err

}

// returns Environment (string) variable or default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}

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

// Creates an array of statuses for zones
func calculateStatuses(zones [][4]int) []zoneStatus {
	result := make([]zoneStatus, len(zones))
	for i, _ := range zones {
		result[i] = calculateStatus(i, zones)
	}
	return result
}

// Calculates status for a zone
func calculateStatus(i int, zones [][4]int) zoneStatus {
	mask := 0x01 << (uint(i) % 16)
	byteIndex := int(math.Floor(float64(i) / 16))
	status := zoneStatus{
		Ready:        false,
		ByPass:       false,
		SysCondition: false,
		InAlarm:      false,
	}
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
func zonesNames(conf Settings) ([]string, error) {
	var data httpRequest
	var names []string
	data.Path = conf.Url + "user/zones.htm"
	data.Params = ""
	data.Method = "GET"
	response, err := makeRequest(data, conf, 2)
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
func doRequest(data httpRequest, conf *Settings, tries int) ([]byte, error) {
	var err error
	var b []byte
	return b, err
}

// HTTP request wrapper. Responsible for all requests. Accept httpRequest struct
// and Settings. Also handles re-try, in case of expired session it may re-login
// if tries is greater than 1
func makeRequest(data httpRequest, conf Settings, tries int) ([]byte, error) {
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
			return makeRequest(data, conf, tries-1)
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
func login(conf Settings) (string, error) {
	var result string
	var session string
	var data httpRequest
	re := regexp.MustCompile(
		`(?msUi)function getSession\(\){return\s"(\S.*)";}`)

	data.Path = conf.Url + "login.cgi"
	data.Params = "lgname=" + conf.User + "&" + "lgpin=" + conf.Pin
	data.Method = "POST"
	response, err := makeRequest(data, conf, 1)

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

// Returns Sequence. Via this request we can retrieve seq.xml response but still
// in order to retrieve Statuses you should use ZoneStatuses function, since
// sequence cannot be used without Zstate.
func sequence(conf Settings) (sequenceReq, error) {
	var data httpRequest
	var result sequenceReq
	data.Path = conf.Url + "user/seq.xml"
	data.Params = addSession("", getSession())
	data.Method = "POST"
	response, err := makeRequest(data, conf, 2)
	if err != nil {
		return result, err
	}
	xml.Unmarshal(response, &result)
	return result, err
}

// Returns Zstate result. Zstate cannot be used without Sequence, only by
// joining Zstate and Sequese requests we can calculate zone statues
// for this use ZoneStatuses()
func zstate(conf Settings, state int) (zstateReq, error) {
	var data httpRequest
	var result zstateReq
	data.Path = conf.Url + "user/zstate.xml"
	data.Method = "POST"
	data.Params = addSession("state="+strconv.Itoa(state), getSession())
	response, err := makeRequest(data, conf, 2)
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
func SetByPass(zone int, conf *Settings) error {
	var data httpRequest
	params := "comm=82&data0=" + strconv.Itoa(zone)
	data.Path = conf.Url + "user/zonefunction.cgi"
	data.Params = addSession(params, getSession())
	data.Method = "POST"
	_, err := doRequest(data, conf, 2)
	return err
}

// Handles system statuses.
func SetSystem(trigger int, conf *Settings) error {
	var params string
	var data httpRequest
	switch trigger {
	case Arm:
		params = "comm=80&data0=2&data2=17&data1=1"
	case Stay:
		params = "comm=80&data0=2&data2=18&data1=1"
	case Disarm:
		params = "comm=80&data0=2&data2=16&data1=1"
	case Chime:
		params = "comm=80&data0=2&data2=1&data1=1"
	default:
		params = "comm=80&data0=2&data2=1&data1=1" // Chime
	}
	data.Path = conf.Url + "user/keyfunction.cgi"
	data.Params = addSession(params, getSession())
	data.Method = "POST"
	_, err := doRequest(data, conf, 2)

	return err
}
