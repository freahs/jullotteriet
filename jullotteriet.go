// Copyright 2015 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Sample twilio demonstrates sending and receiving SMS, receiving calls via Twilio from App Engine flexible environment.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"google.golang.org/appengine"
)

// [START import]
import (
	"bitbucket.org/ckvist/twilio/twiml"
	"bitbucket.org/ckvist/twilio/twirest"
)

// [END import]

func main() {
	http.HandleFunc("/sms/receive", receiveSMSHandler)
	//sendSMS("+46720258512")

	appengine.Main()
}

var (
	twilioClient = twirest.NewClient(
		mustGetenv("TWILIO_ACCOUNT_SID"),
		mustGetenv("TWILIO_AUTH_TOKEN"))
	twilioNumber = mustGetenv("TWILIO_NUMBER")
)

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s environment variable not set.", k)
	}
	return v
}

func sendSMS(to string) {
	if to == "" {
		log.Printf("Missing 'to' parameter.")
		return
	}
	msg := twirest.SendMessage{
		Text: "Hello from App Engine!",
		From: twilioNumber,
		To:   to,
	}

	resp, err := twilioClient.Request(msg)
	if err != nil {
		log.Printf("Could not send SMS: %v", err)
		return
	}

	log.Printf("SMS sent successfully. Response:\n%#v", resp.Message)
}

func receiveSMSHandler(w http.ResponseWriter, r *http.Request) {
	sender := r.FormValue("From")
	body := r.FormValue("Body")
	var num_rows int
	db.QueryRow("SELECT COUNT(*) FROM members WHERE name=$1", name).Scan(&num_rows)
	if num_rows == 0 {
		if _, err := db.Exec("INSERT INTO members VALUES($1, $2)", name, email); err != nil {
			log.Fatalf("Couldn't perform insert. %v", err)
		}
	}

	mod := gmail.ModifyMessageRequest{RemoveLabelIds: []string{"UNREAD"}}
	if _, err := srv.Users.Messages.Modify("me", m.Id, &mod).Do(); err != nil {
		log.Fatalf("Couldn't remove unread label from email. %v", err)
	}

	resp := twiml.NewResponse()
	resp.Action(twiml.Message{
		Body: fmt.Sprintf("Hello, %s, you said: %s", sender, body),
		From: twilioNumber,
		To:   sender,
	})
	resp.Send(w)
}
