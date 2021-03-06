package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/crosstalkio/log"
	"github.com/gorilla/mux"
	"google.golang.org/protobuf/proto"
)

const (
	Accept          = "Accept"
	ContentType     = "Content-Type"
	JsonContentType = "application/json"
)

var (
	ProtobufContentTypes = []string{"application/protobuf", "application/x-protobuf"}
)

type Session struct {
	log.Context
	server         *Server
	Data           map[interface{}]interface{}
	Request        *http.Request
	ResponseWriter http.ResponseWriter
}

func (s *Session) RemoteHost() (net.IP, error) {
	var host string
	var err error
	fwd := s.Request.Header.Get("X-Forwarded-For")
	if fwd != "" {
		splits := strings.Split(fwd, ",")
		host = splits[0]
	} else {
		addr := s.Request.RemoteAddr
		host, _, err = net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, fmt.Errorf("Invalid IP address: %s", host)
	}
	return ip, nil
}

func (s *Session) RequestHeader() http.Header {
	return s.Request.Header
}

func (s *Session) ResponseHeader() http.Header {
	return s.ResponseWriter.Header()
}

func (s *Session) Decode(val interface{}) error {
	switch v := val.(type) {
	case proto.Message:
		if isProto(contentType(s.Request)) ||
			(s.Request.ContentLength <= 0 && isProto(accept(s.Request))) {
			data, err := ioutil.ReadAll(s.Request.Body)
			if err != nil {
				s.Errorf("Failed to read request body: %s", err.Error())
				return err
			}
			err = proto.Unmarshal(data, v)
			if err != nil {
				s.Errorf("Failed to unmarshal proto request body: %s", err.Error())
			}
			return err
		} else {
			err := json.NewDecoder(s.Request.Body).Decode(v)
			if err != nil {
				s.Errorf("Failed to decode JSON request body: %s", err.Error())
				return err
			}
			// to check 'required' props of proto2
			_, err = proto.Marshal(v)
			return err
		}
	default:
		return json.NewDecoder(s.Request.Body).Decode(v)
	}
}

func (s *Session) encode(status int, val interface{}) error {
	switch v := val.(type) {
	case proto.Message:
		accept := accepts(ProtobufContentTypes, s.RequestHeader()[Accept])
		if isProto(contentType(s.Request)) || accept != "" {
			return s.encodeProto(status, v, accept)
		} else {
			return s.encodeJSON(status, v)
		}
	default:
		return s.encodeJSON(status, v)
	}
}

func (s *Session) Status(status int, v interface{}) {
	if isNil(v) {
		s.writeHeader(status)
		return
	}
	var msg string
	switch v := v.(type) {
	case string:
		msg = v
	case error:
		msg = v.Error()
	default:
		err := s.encode(status, v)
		if err != nil {
			s.Errorf("Failed to encode body: %s", err.Error())
		}
		return
	}
	s.ResponseWriter.Header().Set("Content-Type", "text/plain; charset=utf-8")
	s.ResponseWriter.Header().Set("X-Content-Type-Options", "nosniff")
	s.writeHeader(status)
	s.Debugf("Writing text body: %s", msg)
	_, err := fmt.Fprintln(s.ResponseWriter, msg)
	if err != nil {
		s.Errorf("Failed to write text body: %s", err.Error())
	}
}

func (s *Session) StatusCode(code int) {
	s.Status(code, http.StatusText(code))
}

func (s *Session) Statusf(code int, format string, args ...interface{}) {
	s.Status(code, fmt.Sprintf(format, args...))
}

func (s *Session) Vars() map[string]string {
	return mux.Vars(s.Request)
}

func (s *Session) Var(key, preset string) string {
	val := s.Vars()[key]
	if val == "" {
		val = s.Request.FormValue(key)
	}
	if val == "" {
		val = preset
	}
	return val
}

func (s *Session) writeHeader(status int) {
	s.Debugf("Writing header: %d", status)
	s.ResponseWriter.WriteHeader(status)
}

func (s *Session) encodeProto(status int, v proto.Message, accept string) error {
	if accept == "" {
		accept = ProtobufContentTypes[0]
	}
	s.ResponseHeader().Set(ContentType, accept)
	data, err := proto.Marshal(v)
	if err != nil {
		s.Errorf("Failed to encode protobuf: %s", err.Error())
		return err
	}
	s.writeHeader(status)
	s.Debugf("Writing protobuf: %d bytes", len(data))
	_, err = io.Copy(s.ResponseWriter, bytes.NewBuffer(data))
	if err != nil {
		s.Errorf("Failed to write protobuf: %s", err.Error())
		return err
	}
	return nil
}

func (s *Session) encodeJSON(status int, v interface{}) error {
	// s.Debugf("encoing: %v", v)
	s.ResponseHeader().Set(ContentType, JsonContentType)
	var data []byte
	var err error
	if s.server.jsonPrefix != "" || s.server.jsonIndent != "" {
		data, err = json.MarshalIndent(v, s.server.jsonPrefix, s.server.jsonIndent)
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		s.Errorf("Failed to encode JSON: %s", err.Error())
		return err
	}
	s.writeHeader(status)
	s.Debugf("Writing JSON: %d bytes", len(data))
	_, err = s.ResponseWriter.Write(data)
	if err != nil {
		s.Errorf("Failed to write JSON: %s", err.Error())
		return err
	}
	return nil
}

func accept(r *http.Request) string {
	return r.Header.Get(Accept)
}

func contentType(r *http.Request) string {
	return r.Header.Get(ContentType)
}

func isProto(mime string) bool {
	return isTypeOf(mime, ProtobufContentTypes)
}

func isTypeOf(mime string, types []string) bool {
	for _, t := range types {
		if strings.HasPrefix(mime, t) {
			return true
		}
	}
	return false
}

func accepts(types []string, accepts []string) string {
	for _, t := range types {
		for _, a := range accepts {
			if strings.HasPrefix(a, t) {
				return t
			}
		}
	}
	return ""
}
