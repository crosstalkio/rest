package main

import (
	"github.com/crosstalkio/rest"
	"google.golang.org/protobuf/proto"
)

var users = map[string]*User{
	"alice": &User{
		Email:       proto.String("alice@foo.com"),
		DisplayName: proto.String("Alice"),
	},
}

func UserHandler(s *rest.Session) {
	id := s.Var("id", "")
	switch s.Request.Method {
	case "GET":
		user := users[id]
		if user == nil {
			_ = s.Status(404, nil)
			return
		}
		_ = s.Status(200, user)
		return
	case "POST":
		user := &User{}
		err := s.Decode(user)
		if err != nil {
			_ = s.Status(400, nil)
			return
		}
		users[id] = user
	case "DELETE":
		if users[id] == nil {
			_ = s.Status(404, nil)
			return
		}
		delete(users, id)
	default:
		_ = s.Status(405, nil)
		return
	}
}
