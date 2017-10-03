package smshandler

import (
	"bitbucket.org/ckvist/twilio/twiml"
	"bitbucket.org/ckvist/twilio/twirest"
	"net/http"
	"regexp"
	"strings"
)

type handler struct {
	f func(string, string) string
	r *regexp.Regexp
}

var c_map map[string]int
var c_arr []handler

func init() {
	c_map = make(map[string]int)
	RegisterHandler("default", func(string, string) string {
		return "This is a default reply, please replace it with a custom handler."
	})
}

// RegisterHandler takes a command and a handle function. The command should be a valid
// regexp, most commonly a single word command like "START". Be careful not to let the
// regexp eat the whole body of the message though... Matching is case insensitive.
// The handle function handles an incoming SMS matching the command. It takes two
// strings: the number of the sender and the body of the SMS following the command. It
// returns a string which should be sent as a response to the SMS. The body is stripped
// lading and trailing whitespaces
func RegisterHandler(c string, h func(string, string) string) {
	c = strings.ToLower(c)
	if _, ok := c_map[c]; !ok {
		re := regexp.MustCompile(c)
		c_map[c] = len(c_arr)
		c_arr = append(c_arr, handler{h, re})
	} else {
		c_arr[c_map[c]].f = h
	}
}

func HandleSMS(from, body string) string {
	for _, h := range c_arr {
		idx := h.r.FindStringIndex(strings.ToLower(body))
		if idx != nil && idx[0] == 0 {
			return h.f(from, strings.TrimSpace(body[idx[1]:]))
		}
	}
	return c_arr[c_map["default"]].f(from, "")
}

func TwiloSMSHandler(client *twirest.TwilioClient, number string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		from := r.FormValue("From")
		body := r.FormValue("Body")
		resp := twiml.NewResponse()
		resp.Action(twiml.Message{
			From: number,
			To:   from,
			Body: HandleSMS(from, body),
		})
		resp.Send(w)
	}
}
