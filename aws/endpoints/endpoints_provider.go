// Copyright 2020 Baidu, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
// except in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the
// License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing permissions
// and limitations under the License.

package endpoints

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	MinEndpointLength        = 3
	DefaultKeepAliveInterval = 60
	ProbeRequestTimeOut      = 30 * time.Second
	probeKey                 = "lbsdkprobeblacklistbucket20200320/lbsdkprobeblacklistkey20200320"
)

var (
	GEndpoints *GlobalEndpoints
)

func init() {
	GEndpoints = &GlobalEndpoints{
		endpointCollections: make(map[string]*EndpointCollection),
	}
	rand.Seed(time.Now().UTC().UnixNano())
}

type Rgw struct {
	// The ip of endpoint.
	Ip string `xml:"Ip"`

	// The port of endpoint.
	Port string `xml:"Port"`
}

type RgwInfo struct {
	// Metadata about each object returned.
	RgwConfiguration []*Rgw `xml:"Rgw"`

	epoch int
}

// save the info of one endpoint
type SingleEndpoint struct {
	IsInBlackList bool
	Id            uint64
	Protocol      string
	Host          string
	Port          string
	HostAndPort   string
	URL           string
	next          *SingleEndpoint
	pre           *SingleEndpoint
}

// save all endpoints
type EndpointCollection struct {
	numOfActiveEndpoint int
	lastEpoch           int
	keepAliveInterval   int
	validMinEndpointId  uint64
	endpointHead        *SingleEndpoint
	endpointSeed        *[]SingleEndpoint
	blackList           map[string]*SingleEndpoint
	httpClient          *http.Client
	mutex               sync.Mutex
	notify              chan bool
}

// manage all endpoint collections
type GlobalEndpoints struct {
	endpointCollections map[string]*EndpointCollection
	mutex               sync.Mutex
}

// reading endpoints from file
// and creating an new endpoints collection
func NewEndpointCollection(endpointsPath string, keepAliveInterval int) (
	*EndpointCollection, error) {

	if endpointsPath == "" {
		return nil, fmt.Errorf("endpoint path is empty")
	}
	httpClient := NewHttpClient()
	endpoints := &EndpointCollection{
		lastEpoch:         -1,
		keepAliveInterval: keepAliveInterval,
		httpClient:        httpClient,
		blackList:         make(map[string]*SingleEndpoint),
		notify:            make(chan bool),
	}
	if err := endpoints.ReadEndpointsFromFile(endpointsPath, true); err != nil {
		return nil, err
	}
	// start keep alive in the background
	go endpoints.KeepAlive()
	return endpoints, nil
}

// Update the head of EndpointCollection
func (e *EndpointCollection) UpdateWholeEndpoitCollection(head *SingleEndpoint, endpointNum,
	epoch int) error {

	if head == nil || endpointNum == 0 {
		return fmt.Errorf("endpoint list is empty")
	}

	// update the head
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// update min valid endpoint id
	e.endpointHead = head
	e.numOfActiveEndpoint = endpointNum
	// set id
	newValidMinEndpointId := e.validMinEndpointId + 1
	for i := 0; i < endpointNum; i++ {
		head.Id = newValidMinEndpointId
		head = head.next
	}
	e.validMinEndpointId = newValidMinEndpointId
	e.lastEpoch = epoch

	// clear blacklist
	for k := range e.blackList {
		delete(e.blackList, k)
	}

	return nil
}

func (e *EndpointCollection) ReadEndpointsFromFile(endpointPath string, isSeed bool) error {
	var (
		head *SingleEndpoint
	)

	if endpointPath == "" {
		return fmt.Errorf("endpoint path is empty")
	}

	fd, err := os.Open(endpointPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	// start read endpoints from file
	endpointSting := make([]string, 0, 1000)
	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		urlString := strings.TrimSpace(scanner.Text())
		if urlString == "" {
			continue
		} else if strings.HasPrefix(urlString, "#") {
			continue
		} else if len(urlString) < MinEndpointLength {
			continue
		}
		endpointSting = append(endpointSting, urlString)
	}

	activeEndpoint := 0
	endpointAll := make([]SingleEndpoint, len(endpointSting))
	for i, key := range endpointSting {
		if err := parseEndpointFromString(key, &endpointAll[i]); err != nil {
			return err
		}
		head = insertEndpointToHead(&endpointAll[i], head)
		activeEndpoint++
	}

	err = e.UpdateWholeEndpoitCollection(head, activeEndpoint, e.lastEpoch)
	if err != nil {
		return err
	}

	if isSeed {
		e.endpointSeed = &endpointAll
	}
	return nil
}

// must protected by lock
func (e *EndpointCollection) isInActiveEndpoints(endpoint *SingleEndpoint) bool {
	if endpoint == nil || e.endpointHead == nil {
		return false
	}

	if e.endpointHead.URL == endpoint.URL {
		return true
	}

	head := e.endpointHead.next
	for head != endpoint {
		if head.URL == endpoint.URL {
			return true
		}
		head = head.next
	}
	return false
}

func (e *EndpointCollection) insertToEndpointHead(endpoint *SingleEndpoint) bool {
	if endpoint == nil {
		return false
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	if _, ok := e.blackList[endpoint.URL]; ok {
		return false
	}

	if ok := e.isInActiveEndpoints(endpoint); ok {
		return false
	}

	endpoint.next = nil
	endpoint.pre = nil
	endpoint.Id = e.validMinEndpointId
	e.endpointHead = insertEndpointToHead(endpoint, e.endpointHead)
	e.numOfActiveEndpoint++
	return true
}

func (e *EndpointCollection) AddEndpointToBlacklist(endpoint *SingleEndpoint) *SingleEndpoint {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if endpoint == nil || endpoint.IsInBlackList {
		return e.GetRandEndpoint(0)
	}

	e.numOfActiveEndpoint--
	if e.numOfActiveEndpoint <= 0 {
		e.numOfActiveEndpoint = 0
		e.endpointHead = nil
		e.notify <- true
	} else {
		endpoint.next.pre = endpoint.pre
		endpoint.pre.next = endpoint.next
		if endpoint == e.endpointHead {
			e.endpointHead = endpoint.next
		}
	}
	// remove endpoint from endpointHead
	endpoint.next = nil
	endpoint.pre = nil
	endpoint.IsInBlackList = true

	if endpoint.Id >= e.validMinEndpointId {
		e.blackList[endpoint.URL] = endpoint
	}
	return e.GetRandEndpoint(0)
}

// remove endpoint from balcklist and insert to active list
func (e *EndpointCollection) RmEndpointFromBlacklist(host string) bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	endpoint, ok := e.blackList[host]
	if !ok {
		return false
	}

	if !endpoint.IsInBlackList {
		return false
	}

	delete(e.blackList, host)
	endpoint.IsInBlackList = false

	if endpoint.Id >= e.validMinEndpointId {
		e.endpointHead = insertEndpointToHead(endpoint, e.endpointHead)
		e.numOfActiveEndpoint++
		return true
	}
	return false
}

func (e *EndpointCollection) GetNextEndpoint(endpoint *SingleEndpoint) *SingleEndpoint {
	if endpoint == nil || endpoint.next == nil {
		return e.GetRandEndpoint(0)
	} else if endpoint.Id < e.validMinEndpointId || endpoint.IsInBlackList {
		return e.GetRandEndpoint(0)
	}

	// get next valid endpoint
	temp := endpoint.next
	for temp != nil {
		if temp.Id < e.validMinEndpointId || temp.IsInBlackList {
			temp = temp.next
		} else {
			break
		}
	}

	if temp != nil {
		return temp
	} else {
		return e.GetRandEndpoint(0)
	}
}

// get a random endpoint from EndpointCollection
func (e *EndpointCollection) GetRandEndpoint(retryTime int) *SingleEndpoint {
	temp := e.endpointHead
	if temp == nil || e.numOfActiveEndpoint == 0 {
		return nil
	}

	if retryTime <= 0 {
		retryTime = rand.Intn(e.numOfActiveEndpoint)
	}

	for (temp != nil) && (retryTime > 0) {
		temp = temp.next
		retryTime--
	}

	// get next valid endpoint
	for temp != nil {
		if temp.Id < e.validMinEndpointId || temp.IsInBlackList {
			temp = temp.next
		} else {
			break
		}
	}

	if temp == nil {
		return e.endpointHead
	}
	return temp
}

func (e *EndpointCollection) probeEndpoint(URL string) bool {
	// constrcut http request
	httpRequest, err := newHttpRequestFromURL(URL, probeKey, "")
	if err != nil {
		return false
	}
	httpRequest.Method = http.MethodGet
	e.httpClient.Timeout = ProbeRequestTimeOut

	// send probe to endpoint
	httpResponse, err := e.httpClient.Do(httpRequest)
	if err != nil {
		return false
	} else if httpResponse == nil {
		return false
	}

	defer httpResponse.Body.Close()
	io.Copy(ioutil.Discard, httpResponse.Body)
	if httpResponse.StatusCode == 200 || httpResponse.StatusCode == 404 ||
		httpResponse.StatusCode == 403 {
		return true
	}
	return false
}

func (e *EndpointCollection) probeBlacklist() bool {
	blacklists := make([]string, 0, len(e.blackList))
	dellists := make([]string, 0, len(e.blackList))

	{
		e.mutex.Lock()
		for k := range e.blackList {
			endpoint := e.blackList[k]
			if endpoint.Id >= e.validMinEndpointId {
				blacklists = append(blacklists, k)
			} else {
				dellists = append(dellists, k)
			}
		}
		for _, k := range dellists {
			delete(e.blackList, k)
		}
		e.mutex.Unlock()
	}

	success := false
	for _, key := range blacklists {
		if ok := e.probeEndpoint(key); ok {
			e.RmEndpointFromBlacklist(key)
			success = true
		}
	}

	return success
}

func (e *EndpointCollection) probeEndpointFromSeed() bool {
	if e.endpointSeed == nil {
		return false
	}

	success := false
	for _, endpoint := range *e.endpointSeed {
		if _, ok := e.blackList[endpoint.URL]; ok {
			continue
		}

		if ok := e.probeEndpoint(endpoint.URL); ok {
			e.insertToEndpointHead(&endpoint)
			success = true
		}
	}
	return success
}

// Download endpoint list from server
// decode the endpoint list into RgwInfo
func (e *EndpointCollection) getRgwInfoFromServer(endpoint *SingleEndpoint) (*RgwInfo, error) {
	httpRequest, err := newHttpRequestFromURL(endpoint.URL, "/", "rgw")
	if err != nil {
		return nil, err
	}
	httpRequest.Method = http.MethodGet
	e.httpClient.Timeout = ProbeRequestTimeOut

	// send http request to endpoint
	httpResponse, err := e.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	} else if httpResponse == nil {
		return nil, fmt.Errorf("response is empty!")
	}

	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != 200 {
		message, _ := ioutil.ReadAll(httpResponse.Body)
		return nil, fmt.Errorf(string(message))
	}
	epochs, ok := httpResponse.Header["Last-Epoch"]
	if !ok {
		return nil, fmt.Errorf("epoch not in response headers")
	} else if len(epochs) != 1 {
		return nil, fmt.Errorf("the length of epoch %d are invaild", len(epochs))
	}

	epoch, err := strconv.Atoi(epochs[0])
	if err != nil {
		return nil, err
	}

	// decode xml
	rgws := &RgwInfo{}
	body, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	err = xml.Unmarshal(body, rgws)
	if err != nil {
		return nil, err
	}

	if rgws.RgwConfiguration == nil || len(rgws.RgwConfiguration) == 0 {
		return nil, fmt.Errorf("RgwConfiguration is empty")
	}
	rgws.epoch = epoch

	return rgws, nil
}

// Parse RgwInfo to a list of SingleEndpoint
func (e *EndpointCollection) ParseEndpointFromRgwInfo(rgws *RgwInfo) (*SingleEndpoint, int) {
	var (
		head *SingleEndpoint
	)
	endpointAll := make([]SingleEndpoint, len(rgws.RgwConfiguration))
	currentId := 0
	for _, rgw := range rgws.RgwConfiguration {
		if rgw.Ip == "" {
			continue
		}
		err := parseEndpointFromString(rgw.Ip+":"+rgw.Port, &endpointAll[currentId])
		if err != nil {
			continue
		}
		head = insertEndpointToHead(&endpointAll[currentId], head)
		currentId++
	}
	return head, currentId
}

func (e *EndpointCollection) UpdateEndpointsByEndpoint(endpoint *SingleEndpoint,
	forceUpdate bool) bool {
	// get the information of rgws from endpoint
	rgws, err := e.getRgwInfoFromServer(endpoint)
	if err != nil {
		return false
	}

	// check epoch
	if !forceUpdate && rgws.epoch <= e.lastEpoch {
		return true
	}

	head, rgwNum := e.ParseEndpointFromRgwInfo(rgws)
	if head == nil || rgwNum == 0 {
		return false
	}

	if err := e.UpdateWholeEndpoitCollection(head, rgwNum, rgws.epoch); err == nil {
		return true
	}

	return false
}

func (e *EndpointCollection) UpdateEndpointByApi() bool {
	retry := e.numOfActiveEndpoint
	endpoint := e.endpointHead

	for endpoint != nil && retry >= 0 {
		endpoint = endpoint.next
		if endpoint.IsInBlackList || endpoint.Id < e.validMinEndpointId {
			continue
		}
		retry--

		if ok := e.UpdateEndpointsByEndpoint(endpoint, false); ok {
			return true
		}
	}
	return false
}

func (e *EndpointCollection) UpdateEndpointFromSeed() bool {
	if e.endpointSeed == nil {
		return false
	}

	for _, endpoint := range *e.endpointSeed {
		if _, ok := e.blackList[endpoint.URL]; ok {
			continue
		}

		if ok := e.UpdateEndpointsByEndpoint(&endpoint, true); ok {
			return true
		}
	}
	return false
}

// 1. get endpoint list from server background
// 2. probe the endpoint in blacklist
func (e *EndpointCollection) KeepAlive() {
	var (
		before time.Duration
		after  time.Duration
	)
	if e.keepAliveInterval > 1 {
		before = 1
		after = time.Duration(e.keepAliveInterval - 1)
	} else {
		before = 1
		after = 0
	}

	immediately := false
	for {
		// sleep
		if before > 0 && !immediately {
			select {
			case <-e.notify:
				immediately = true
			case <-time.After(before * time.Second):
			}
		}

		ok := e.UpdateEndpointByApi()
		if !ok {
			ok = e.probeBlacklist()
		}

		if e.numOfActiveEndpoint == 0 {
			// all endpoints have been added into blacklist and are invalid
			ok = e.UpdateEndpointFromSeed()
			if !ok {
				ok = e.probeEndpointFromSeed()
			}
		}

		if immediately && ok {
			immediately = false
		} else if immediately {
			// failed to fetch endpoint list from server
			// retry immediately
			time.Sleep(1 * time.Second)
			continue
		}

		// sleep
		if after > 0 && !immediately {
			select {
			case <-e.notify:
				immediately = true
			case <-time.After(after * time.Second):
			}
		}
	}
}

// the endpoint collection
func (g *GlobalEndpoints) FindEndpointCollection(endpointsPath string,
	keepAliveInterval int) (*EndpointCollection, error) {

	g.mutex.Lock()
	defer g.mutex.Unlock()
	endpoints, ok := g.endpointCollections[endpointsPath]
	if ok {
		return endpoints, nil
	}

	endpoints, err := NewEndpointCollection(endpointsPath, keepAliveInterval)
	if err != nil {
		return nil, err
	}
	g.endpointCollections[endpointsPath] = endpoints
	return endpoints, nil
}

func parseEndpointFromString(urlString string, endpoint *SingleEndpoint) error {
	if !strings.Contains(urlString, "//") {
		urlString = "//" + urlString
	}

	parseResult, err := url.Parse(urlString)
	if err != nil {
		return err
	}

	endpoint.Host = parseResult.Hostname()
	endpoint.Port = parseResult.Port()

	if parseResult.Scheme != "" {
		endpoint.Protocol = parseResult.Scheme
	} else if endpoint.Port == "443" {
		endpoint.Protocol = "https"
	} else {
		endpoint.Protocol = "http"
	}

	if endpoint.Port != "" {
		endpoint.HostAndPort = endpoint.Host + ":" + endpoint.Port
	}
	endpoint.URL = fmt.Sprintf("%s://%s", endpoint.Protocol, endpoint.HostAndPort)

	return nil
}

func insertEndpointToHead(endpoint *SingleEndpoint, head *SingleEndpoint) *SingleEndpoint {
	if endpoint == nil {
		return head
	}

	if head == nil {
		head = endpoint
		head.next = head
		head.pre = head
	} else if endpoint.HostAndPort < head.HostAndPort {
		endpoint.pre = head.pre
		endpoint.next = head
		head.pre.next = endpoint
		head.pre = endpoint
		head = endpoint
	} else {
		temp := head.next
		for endpoint.HostAndPort > temp.HostAndPort && temp != head {
			temp = temp.next
		}
		endpoint.pre = temp.pre
		endpoint.next = temp
		temp.pre.next = endpoint
		temp.pre = endpoint
	}
	return head
}
