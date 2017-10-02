package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

import (
	"bitbucket.org/ckvist/twilio/twiml"
	"bitbucket.org/ckvist/twilio/twirest"
	"database/sql"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

var (
	twilioClient = twirest.NewClient(
		mustGetenv("TWILIO_ACCOUNT_SID"),
		mustGetenv("TWILIO_AUTH_TOKEN"))
	twilioNumber = mustGetenv("TWILIO_NUMBER")
	db           *sql.DB
)

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s environment variable not set.", k)
	}
	return v
}

func main() {
	var err error
	db, err = sql.Open("postgres", mustGetenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
	}
	//sendSMS("+46720258512")

	r := mux.NewRouter()
	r.HandleFunc("/", defaultHandler)
	r.HandleFunc("/sms/receive", receiveSMSHandler)

	// Bind to a port and pass our router in
	log.Fatal(http.ListenAndServe(":"+mustGetenv("PORT"), r))
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Nothing here\n"))
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

func addMember(name, number string) {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS members (name VARCHAR NOT NULL PRIMARY KEY, number VARCHAR)"); err != nil {
		log.Printf("Error creating database table: %q", err)
		return
	}
}

func receiveSMSHandler(w http.ResponseWriter, r *http.Request) {
	sender := r.FormValue("From")
	body := r.FormValue("Body")
	/*
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
	*/
	resp := twiml.NewResponse()
	resp.Action(twiml.Message{
		Body: fmt.Sprintf("Hello, %s, you said: %s", sender, body),
		From: twilioNumber,
		To:   sender,
	})
	resp.Send(w)
}
