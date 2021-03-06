package rest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/crosstalkio/log"
	"google.golang.org/protobuf/proto"
)

const (
	contentTypeHeader = "Content-Type"
	jsonContentType   = "application/json"
	protoContentType  = "application/protobuf"
)

type Auth interface {
	Authorize(req *http.Request) error
	Validate(res *Response) (bool, error)
	Invalidate() error
}

type Request struct {
	client *Client
	url    *URL
	header http.Header
}

func (r *Request) Header(name, value string) *Request {
	r.header.Set(name, value)
	return r
}

func (r *Request) Join(path string) *Request {
	r.url.Join(path)
	return r
}

func (r *Request) Param(name, value string) *Request {
	r.url.Param(name, value)
	return r
}

func (r *Request) Do(method string, v interface{}) (*Response, error) {
	url := r.url.Encode()
	return r.client.request(method, r.header, url, v)
}

func (r *Request) Get() (*Response, error) {
	return r.Do("GET", nil)
}

func (r *Request) Post(v interface{}) (*Response, error) {
	return r.Do("POST", v)
}

func (r *Request) Put(v interface{}) (*Response, error) {
	return r.Do("PUT", v)
}

func (r *Request) Delete() (*Response, error) {
	return r.Do("DELETE", nil)
}

type Response struct {
	log.Sugar
	*http.Response
	Body     []byte
	protobuf bool
}

func (r *Response) Decode(val interface{}) error {
	var err error
	protobuf := false
	switch val := val.(type) {
	case proto.Message:
		if r.protobuf {
			protobuf = true
			err = proto.Unmarshal(r.Body, val)
		} else {
			err = json.Unmarshal(r.Body, val)
		}
	default:
		err = json.Unmarshal(r.Body, val)
	}
	if err != nil && !protobuf && r.Response.Header.Get(contentTypeHeader) != jsonContentType {
		err = fmt.Errorf("%s", r.Body)
	}
	return err
}

type Client struct {
	log.Sugar
	Client   *http.Client
	URL      string
	auth     Auth
	protobuf bool
	status   int
}

func NewClient(logger log.Logger, timeout time.Duration) *Client {
	return &Client{
		Sugar:  log.NewSugar(logger),
		Client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) Auth(auth Auth) *Client {
	c.auth = auth
	return c
}

func (c *Client) Protobuf() *Client {
	c.protobuf = true
	return c
}

func (c *Client) ExpectStatus(status int) *Client {
	c.status = status
	return c
}

func (c *Client) New(u string) *Request {
	return &Request{client: c, url: NewURL(u), header: make(http.Header)}
}

func (c *Client) Get(url string) (*Response, error) {
	return c.Request("GET", url, nil)
}

func (c *Client) Post(url string, req interface{}) (*Response, error) {
	return c.Request("POST", url, req)
}

func (c *Client) Put(url string, req interface{}) (*Response, error) {
	return c.Request("PUT", url, req)
}

func (c *Client) Delete(url string) (*Response, error) {
	return c.Request("DELETE", url, nil)
}

func (c *Client) Request(method, url string, r interface{}) (*Response, error) {
	start := time.Now()
	res, err := c.request(method, nil, url, r)
	if err != nil {
		return nil, err
	}
	if c.auth != nil {
		valid, err := c.auth.Validate(res)
		if err != nil {
			c.Errorf("Failed to validate auth: %s", err.Error())
			return nil, err
		}
		if !valid {
			err = c.auth.Invalidate()
			if err != nil {
				c.Errorf("Failed to invalidate auth: %s", err.Error())
				return nil, err
			}
			res, err = c.request(method, nil, url, r)
			if err != nil {
				return nil, err
			}
		}
	}
	c.Infof("Requested in %v: %s %s", time.Since(start), method, url)
	return res, nil
}

func (c *Client) request(method string, header http.Header, url string, r interface{}) (*Response, error) {
	var body []byte
	var err error
	var ctype string
	if !isNil(r) {
		t := reflect.ValueOf(r)
		if t.Kind() == reflect.Slice && t.Type() == reflect.TypeOf([]byte(nil)) {
			body = r.([]byte)
		} else {
			switch r := r.(type) {
			case io.Reader:
				body, err = ioutil.ReadAll(r)
				if err != nil {
					c.Errorf("Failed to read request body: %s", err.Error())
					return nil, err
				}
			case json.RawMessage:
				ctype = jsonContentType
				body = r
			case proto.Message:
				if c.protobuf {
					ctype = protoContentType
					body, err = proto.Marshal(r)
				} else {
					ctype = jsonContentType
					body, err = json.Marshal(r)
				}
			default:
				ctype = jsonContentType
				body, err = json.Marshal(r)
			}
			if err != nil {
				c.Errorf("Failed to marshal: %s", err.Error())
				return nil, err
			}
		}
	}
	req, err := http.NewRequest(method, c.URL+url, bytes.NewBuffer(body))
	if err != nil {
		c.Errorf("Failed to create request: %s", err.Error())
		return nil, err
	}
	if ctype != "" {
		req.Header.Set(contentTypeHeader, ctype)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if c.auth != nil {
		auth := c.auth
		c.auth = nil
		err := auth.Authorize(req)
		if err != nil {
			c.auth = auth
			c.Errorf("Failed to authorize: %s", err.Error())
			return nil, err
		}
		c.auth = auth
	}
	if ctype == "" {
		if c.protobuf {
			req.Header.Set("Accept", protoContentType)
		} else {
			req.Header.Set("Accept", jsonContentType)
		}
	}
	c.dumpRequest(req, body)
	res, err := c.Client.Do(req)
	if err != nil {
		c.Errorf("Failed to make request: %s", err.Error())
		return nil, err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		c.Errorf("Failed to read body: %s", err.Error())
		return nil, err
	}
	c.dumpResponse(res, data)
	if c.status != 0 && c.status != res.StatusCode {
		c.Errorf("Unepxected status code: %d", res.StatusCode)
		return nil, errors.New(res.Status)
	}
	res.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	return &Response{
		Sugar:    c.Sugar,
		Response: res,
		Body:     data,
		protobuf: c.protobuf,
	}, nil
}

func (c *Client) dumpRequest(req *http.Request, data []byte) {
	dump := &struct {
		Method   string                 `json:"method"`
		URL      string                 `json:"url"`
		Protocol string                 `json:"protocol"`
		Headers  map[string]interface{} `json:"headers"`
		Body     interface{}            `json:"body,omitempty"`
	}{
		Method:   req.Method,
		URL:      req.URL.RequestURI(),
		Protocol: req.Proto,
		Headers:  make(map[string]interface{}),
	}
	for k, v := range req.Header {
		if len(v) == 1 {
			dump.Headers[k] = v[0]
		} else {
			dump.Headers[k] = v
		}
	}
	if len(data) > 0 {
		ctype := req.Header.Get(contentTypeHeader)
		if strings.HasPrefix(ctype, jsonContentType) {
			dump.Body = json.RawMessage(data)
		} else if strings.HasPrefix(ctype, "text/") {
			dump.Body = string(data)
		}
	}
	bytes, err := json.Marshal(dump)
	if err == nil {
		c.Debugf("%s", bytes)
	}
}

func (c *Client) dumpResponse(res *http.Response, data []byte) {
	dump := &struct {
		Status   string                 `json:"status"`
		Protocol string                 `json:"protocol"`
		Headers  map[string]interface{} `json:"headers"`
		Body     interface{}            `json:"body,omitempty"`
	}{
		Status:   res.Status,
		Protocol: res.Proto,
		Headers:  make(map[string]interface{}),
	}
	for k, v := range res.Header {
		if len(v) == 1 {
			dump.Headers[k] = v[0]
		} else {
			dump.Headers[k] = v
		}
	}
	if len(data) > 0 {
		ctype := res.Header.Get(contentTypeHeader)
		if strings.HasPrefix(ctype, jsonContentType) {
			dump.Body = json.RawMessage(data)
		} else if strings.HasPrefix(ctype, "text/") {
			dump.Body = string(data)
		}
	}
	bytes, err := json.Marshal(dump)
	if err == nil {
		c.Debugf("%s", bytes)
		return
	}
}
