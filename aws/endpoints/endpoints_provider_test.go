package endpoints

import (
	"fmt"
	"os"
	"testing"
)

const (
	TEST_ENDPOINT_PATH = "./test_endpoints_path123"
)

var (
	numOfActiveEndpoint int
)

func init() {
	// write endpoints
	fd, err := os.Create(TEST_ENDPOINT_PATH)
	if err != nil {
		fmt.Println("failed write to endpoint path", TEST_ENDPOINT_PATH)
		return
	}
	defer fd.Close()
	fd.WriteString("http://abc1.test:8080\n")
	fd.WriteString("http://abc2.test:8080\n")
	fd.WriteString("http://abc3.test:8080\n")
	numOfActiveEndpoint = 3
}

func TestInitEndpointProvider(t *testing.T) {
	if GEndpoints == nil {
		t.Errorf("expected GEndpoints != nil, got nil")
	}
}

func TestNewEndpointCollection(t *testing.T) {
	keepAliveInterval := 3

	_, err := NewEndpointCollection(TEST_ENDPOINT_PATH, -1)
	if err == nil {
		t.Fatalf("1 expect error, got nil")
	}

	_, err = NewEndpointCollection("./dont_exist3434", 0)
	if err == nil {
		t.Fatalf("2 expect error, got nil")
	}

	ec, err := NewEndpointCollection(TEST_ENDPOINT_PATH, keepAliveInterval)
	if err != nil {
		t.Fatalf("3 expect nil, got err != nil")
	}

	if ec.numOfActiveEndpoint != numOfActiveEndpoint {
		t.Errorf("4 expect %d, got %d", numOfActiveEndpoint, ec.numOfActiveEndpoint)
	}

	if ec.lastEpoch != -1 {
		t.Errorf("5 expect %d, got %d", -1, ec.lastEpoch)
	}

	if ec.keepAliveInterval != keepAliveInterval {
		t.Errorf("6 expect %d, got %d", keepAliveInterval, ec.keepAliveInterval)
	}

	if ec.validMinEndpointId != 1 {
		t.Errorf("7 expect 1, got %d", ec.validMinEndpointId)
	}

	if ec.endpointHead == nil {
		t.Errorf("9 expect endpointHead != nil")
	}

	if ec.endpointSeed == nil {
		t.Errorf("10 expect endpointSeed != nil")
	}

	if len(*ec.endpointSeed) != numOfActiveEndpoint {
		t.Errorf("11 expect len(endpointSeed) = %d, got %d",
			numOfActiveEndpoint, len(*ec.endpointSeed))
	}

	// http://abc1.test:8080
	// http://abc2.test:8080
	// http://abc3.test:8080

	endpoint1 := &SingleEndpoint{
		URL: "http://abc1.test:8080",
	}
	endpoint2 := &SingleEndpoint{
		URL: "http://abc2.test:8080",
	}
	endpoint3 := &SingleEndpoint{
		URL: "http://abc3.test:8080",
	}
	if !ec.isInActiveEndpoints(endpoint1) {
		t.Errorf("12 expect http://abc1.test:8080 in EndpointCollection")
	}
	if !ec.isInActiveEndpoints(endpoint2) {
		t.Errorf("13 expect http://abc2.test:8080 in EndpointCollection")
	}
	if !ec.isInActiveEndpoints(endpoint3) {
		t.Errorf("14 expect http://abc3.test:8080 in EndpointCollection")
	}
}

func TestUpdateWholeEndpoitCollection(t *testing.T) {
	ec, err := NewEndpointCollection(TEST_ENDPOINT_PATH, 3)
	if err != nil {
		t.Fatalf("1 expect nil, got err != nil")
	}

	endpoint1 := &SingleEndpoint{
		URL: "http://abc4.test:8080",
	}
	endpoint2 := &SingleEndpoint{
		URL: "http://abc5.test:8080",
	}

	endpoint1.next = endpoint2
	endpoint1.pre = endpoint2
	endpoint2.next = endpoint1
	endpoint2.pre = endpoint1

	if ec.lastEpoch != -1 {
		t.Errorf("expect lastEpoch %d, got %d", -1, ec.lastEpoch)
	}

	ec.UpdateWholeEndpoitCollection(endpoint1, 2, 34)

	endpoint3 := &SingleEndpoint{
		URL: "http://abc1.test:8080",
	}
	endpoint4 := &SingleEndpoint{
		URL: "http://abc2.test:8080",
	}
	endpoint5 := &SingleEndpoint{
		URL: "http://abc3.test:8080",
	}
	if ec.isInActiveEndpoints(endpoint3) {
		t.Errorf("2 expect http://abc1.test:8080 not in EndpointCollection")
	}
	if ec.isInActiveEndpoints(endpoint4) {
		t.Errorf("3 expect http://abc2.test:8080 not in EndpointCollection")
	}
	if ec.isInActiveEndpoints(endpoint5) {
		t.Errorf("4 expect http://abc3.test:8080 not in EndpointCollection")
	}
	if !ec.isInActiveEndpoints(endpoint1) {
		t.Errorf("5 expect http://abc4.test:8080 in EndpointCollection")
	}
	if !ec.isInActiveEndpoints(endpoint2) {
		t.Errorf("6 expect http://abc5.test:8080 in EndpointCollection")
	}
	if ec.lastEpoch != 34 {
		t.Errorf("7 expect lastEpoch %d, got %d", 34, ec.lastEpoch)
	}
	if ec.validMinEndpointId != 2 {
		t.Errorf("8 expect validMinEndpointId %d, got %d", 2, ec.validMinEndpointId)
	}
	if endpoint1.Id != 2 {
		t.Errorf("9 expect ID %d, got %d", 2, endpoint1.Id)
	}
	if endpoint2.Id != 2 {
		t.Errorf("10 expect ID %d, got %d", 2, endpoint2.Id)
	}
}

func TestBlacklist(t *testing.T) {
	ec, err := NewEndpointCollection(TEST_ENDPOINT_PATH, 3)
	if err != nil {
		t.Fatalf("1 expect nil, got err != nil")
	}

	endpoint1 := &SingleEndpoint{
		URL: "http://abc4.test:8080",
		Id:  0,
	}

	endpoint2 := &SingleEndpoint{
		URL: "http://abc5.test:8080",
	}

	endpoint3 := ec.endpointHead
	endpoint4 := endpoint3.next
	endpoint5 := endpoint4.next

	endpoint1.next = endpoint2
	endpoint1.pre = endpoint2
	endpoint2.next = endpoint1
	endpoint2.pre = endpoint1

	endpoint := ec.AddEndpointToBlacklist(endpoint1)
	if endpoint == nil {
		t.Fatalf("2 expect not nil, got err == nil")
	}

	if _, ok := ec.blackList[endpoint.URL]; ok {
		t.Fatalf("3 expect not in blacklist")
	}

	if endpoint1.next != nil || endpoint1.pre != nil {
		t.Fatalf("4 expect endpoint1.next == nil && endpoint1.pre == nil")
	}

	if endpoint2.next != endpoint2 || endpoint2.pre != endpoint2 {
		t.Fatalf("5 expect endpoint2.next == endpoint2 && endpoint1.pre == endpoint2")
	}

	if endpoint1.IsInBlackList != true {
		t.Fatalf("6 expect endpoint1.IsInBlackList == true")
	}

	if !ec.isInActiveEndpoints(endpoint3) {
		t.Errorf("7 expect endpoint in EndpointCollection")
	}

	if !ec.isInActiveEndpoints(endpoint4) {
		t.Errorf("8 expect endpoint in EndpointCollection")
	}

	if !ec.isInActiveEndpoints(endpoint5) {
		t.Errorf("9 expect endpoint in EndpointCollection")
	}

	endpoint = ec.AddEndpointToBlacklist(endpoint3)
	if endpoint == nil {
		t.Fatalf("10 expect not nil, got err == nil")
	}

	if ec.isInActiveEndpoints(endpoint3) {
		t.Errorf("11 expect endpoint not in EndpointCollection")
	}

	if ec.numOfActiveEndpoint != 2 {
		t.Errorf("12 expect numOfActiveEndpoint 2, got %d", ec.numOfActiveEndpoint)
	}

	endpoint = ec.AddEndpointToBlacklist(endpoint4)
	if endpoint == nil {
		t.Fatalf("13 expect not nil, got err == nil")
	}

	if ec.isInActiveEndpoints(endpoint4) {
		t.Errorf("14 expect endpoint not in EndpointCollection")
	}

	if ec.numOfActiveEndpoint != 1 {
		t.Errorf("15 expect numOfActiveEndpoint 1, got %d", ec.numOfActiveEndpoint)
	}

	endpoint = ec.AddEndpointToBlacklist(endpoint5)
	if endpoint != nil {
		t.Fatalf("151 expect nil, got endpoint != nil")
	}

	if ec.isInActiveEndpoints(endpoint5) {
		t.Errorf("152 expect endpoint not in EndpointCollection")
	}

	if ec.numOfActiveEndpoint != 0 {
		t.Errorf("153 expect numOfActiveEndpoint 0, got %d", ec.numOfActiveEndpoint)
	}

	ok := ec.RmEndpointFromBlacklist(endpoint1.URL)
	if ok {
		t.Fatalf("17 expect ok == false")
	}

	ok = ec.RmEndpointFromBlacklist(endpoint5.URL)
	if !ok {
		t.Fatalf("18 expect ok == true")
	}

	if !ec.isInActiveEndpoints(endpoint5) {
		t.Errorf("19 expect endpoint in EndpointCollection")
	}

	if ec.numOfActiveEndpoint != 1 {
		t.Errorf("20 expect numOfActiveEndpoint 1, got %d", ec.numOfActiveEndpoint)
	}

	if ok = ec.RmEndpointFromBlacklist(endpoint4.URL); !ok {
		t.Fatalf("21 expect ok == true")
	}

	if !ec.isInActiveEndpoints(endpoint4) {
		t.Errorf("22 expect endpoint in EndpointCollection")
	}

	if ec.numOfActiveEndpoint != 2 {
		t.Errorf("23 expect numOfActiveEndpoint 2, got %d", ec.numOfActiveEndpoint)
	}

	if ok = ec.RmEndpointFromBlacklist(endpoint3.URL); !ok {
		t.Fatalf("24 expect ok == true")
	}

	if !ec.isInActiveEndpoints(endpoint3) {
		t.Errorf("25 expect endpoint in EndpointCollection")
	}

	if ec.numOfActiveEndpoint != 3 {
		t.Errorf("26 expect numOfActiveEndpoint 3, got %d", ec.numOfActiveEndpoint)
	}

	if ec.endpointHead == nil {
		t.Errorf("27 expect endpointHead not nil")
	}
}

func TestGetNext(t *testing.T) {
	ec, err := NewEndpointCollection(TEST_ENDPOINT_PATH, 3)
	if err != nil {
		t.Fatalf("1 expect nil, got err != nil")
	}

	endpoint1 := &SingleEndpoint{
		URL: "http://abc4.test:8080",
		Id:  0,
	}

	endpoint2 := &SingleEndpoint{
		URL:           "http://abc5.test:8080",
		IsInBlackList: true,
	}

	endpoint := ec.GetNextEndpoint(endpoint1)
	if !ec.isInActiveEndpoints(endpoint) {
		t.Errorf("2 expect endpoint in EndpointCollection")
	}

	endpoint1.next = endpoint2
	endpoint1.pre = endpoint2
	endpoint2.next = endpoint1
	endpoint2.pre = endpoint1
	endpoint = ec.GetNextEndpoint(endpoint1)
	if !ec.isInActiveEndpoints(endpoint) {
		t.Errorf("3 expect endpoint in EndpointCollection")
	}

	endpoint3 := ec.endpointHead
	endpoint4 := endpoint3.next
	endpoint5 := endpoint4.next

	endpoint3.IsInBlackList = true
	endpoint4.IsInBlackList = true

	endpoint = ec.GetNextEndpoint(endpoint5)
	if endpoint == nil {
		t.Fatalf("4 expect not nil, got err == nil")
	}
	if !ec.isInActiveEndpoints(endpoint) {
		t.Errorf("5 expect endpoint in EndpointCollection")
	}
	if endpoint.URL != endpoint5.URL {
		t.Errorf("5 expect endpoint.URL = %s, got %s", endpoint5.URL, endpoint.URL)
	}
}

func TestInsertToEndpointHead(t *testing.T) {
	ec, err := NewEndpointCollection(TEST_ENDPOINT_PATH, 3)
	if err != nil {
		t.Fatalf("1 expect nil, got err != nil")
	}

	endpoint1 := &SingleEndpoint{
		URL: "http://abc4.test:8080",
		Id:  0,
	}

	endpoint3 := ec.endpointHead
	endpoint4 := endpoint3.next

	ec.AddEndpointToBlacklist(endpoint3)
	ok := ec.insertToEndpointHead(endpoint3)
	if ok {
		t.Fatalf("2 expect ok == false, got ok == true")
	}

	ok = ec.insertToEndpointHead(endpoint4)
	if ok {
		t.Fatalf("3 expect ok == false, got ok == true")
	}
	ok = ec.insertToEndpointHead(endpoint1)
	if !ok {
		t.Fatalf("4 expect ok != false, got ok != true")
	}

	if ec.numOfActiveEndpoint != 3 {
		t.Errorf("5 expect numOfActiveEndpoint %d, got %d", 3, ec.numOfActiveEndpoint)
	}

	if endpoint1.Id != endpoint3.Id {
		t.Errorf("6 expect Id %d, got %d", endpoint3.Id, endpoint1.Id)
	}
}

func TestFindEndpointCollection(t *testing.T) {
	_, err := GEndpoints.FindEndpointCollection("./dsfdsfdsfdsfdsf", 1)
	if err == nil {
		t.Fatalf("1 expect err != nil, got err == nil")
	}

	ec, err := GEndpoints.FindEndpointCollection(TEST_ENDPOINT_PATH, 1)
	if err != nil {
		t.Fatalf("2 expect err == nil, got err != nil")
	}

	ec1, err := GEndpoints.FindEndpointCollection(TEST_ENDPOINT_PATH, 1)
	if err != nil {
		t.Fatalf("3 expect err == nil, got err != nil")
	}

	if ec != ec1 {
		t.Fatalf("4 expect ec1 = %v, got %v", ec, ec1)
	}

}

func TestProbeEndpoint(t *testing.T) {
	ec, err := GEndpoints.FindEndpointCollection(TEST_ENDPOINT_PATH, 1)
	if err != nil {
		t.Fatalf("1 expect err == nil, got err != nil")
	}

	cases := map[string]struct {
		URL string
		ret bool
	}{
		"error 1": {
			ret: false,
		},
		"error 2": {
			URL: "http://www.baidu.comxxx",
			ret: false,
		},
		"error 3": {
			URL: "http://127.0.0.1:808", // port 808 should not listen
			ret: false,
		},
		"error 4": {
			URL: "http://127.0.0.1:22",
			ret: false,
		},
		"success": {
			URL: "http://10.130.48.88:8080", // rgw should listen on port 8080
			ret: true,
		},
	}

	for name, c := range cases {
		ok := ec.probeEndpoint(c.URL)
		if ok != c.ret {
			t.Errorf("%s expect %v, got %v", name, c.ret, ok)
		}
	}
}
