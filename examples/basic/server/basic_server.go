package main

import (
	. "github.com/zubairhamed/canopus"
)

func main() {
	server := NewLocalServer()

	server.Get("/hello", func(req *Request) *Response {
		msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
		msg.SetStringPayload("Acknowledged: " + req.GetMessage().Payload.String())
		res := NewResponse(msg, nil)

		return res
	})

	server.Get("/basic", func(req *Request) *Response {
		msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
		msg.SetStringPayload("Acknowledged")

		res := NewResponse(msg, nil)

		return res
	})

	server.Get("/basic/json", func(req *Request) *Response {
		msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
		res := NewResponse(msg, nil)

		return res
	})

	server.Get("/basic/xml", func(req *Request) *Response {
		msg := NewMessageOfType(TYPE_ACKNOWLEDGEMENT, req.GetMessage().MessageId)
		res := NewResponse(msg, nil)

		return res
	})

	server.Start()
}
