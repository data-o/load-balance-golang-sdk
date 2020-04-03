// Copyright 2017 Baidu, Inc.
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
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultMaxIdleConnsPerHost   = 500
	defaultMaxIdleConns          = 1000
	defaultIdleConnTimeout       = 0
	defaultDialTimeout           = 30 * time.Second
	defaultResponseHeaderTimeout = 20 * time.Second
)

// build a new http client
func NewHttpClient() *http.Client {
	httpClient := &http.Client{}
	transport := &http.Transport{
		MaxIdleConns:          defaultMaxIdleConns,
		MaxIdleConnsPerHost:   defaultMaxIdleConnsPerHost,
		IdleConnTimeout:       defaultIdleConnTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
		//DisableKeepAlives: true,
		Dial: func(network, address string) (net.Conn, error) {
			conn, err := net.DialTimeout(network, address, defaultDialTimeout)
			if err != nil {
				return nil, err
			}
			return conn, nil
		},
	}
	httpClient.Transport = transport

	return httpClient
}

func newHttpRequestFromEndpoint(endpoint *SingleEndpoint, path, params string) *http.Request {
	httpRequest := &http.Request{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	internalUrl := &url.URL{
		Scheme:   endpoint.Protocol,
		Host:     endpoint.HostAndPort,
		Path:     path,
		RawQuery: params,
	}
	httpRequest.URL = internalUrl
	return httpRequest
}

func newHttpRequestFromURL(URL, path, params string) (*http.Request, error) {
	httpRequest := &http.Request{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	internalUrl, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}
	internalUrl.Path = path
	internalUrl.RawQuery = params
	httpRequest.URL = internalUrl
	return httpRequest, nil
}
