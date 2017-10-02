package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
	r.HandleFunc("/test", testHandler)
	r.HandleFunc("/sms/receive", receiveSMSHandler)

	// Bind to a port and pass our router in
	log.Fatal(http.ListenAndServe(":"+mustGetenv("PORT"), r))
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Nothing here\n"))
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	insert := addMember("test", "asdf")
	fmt.Fprintf(w, "%v", insert)

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

func addMember(name, number string) bool {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS members (number VARCHAR NOT NULL PRIMARY KEY, name VARCHAR)"); err != nil {
		log.Printf("addMember: %q", err)
		return false
	}

	var num_rows int
	if err := db.QueryRow("SELECT count(*) FROM members WHERE number=$1", number).Scan(&num_rows); err != nil {
		log.Printf("addMember: %q", err)
		return false
	}

	if num_rows != 0 {
		return false
	}

	if _, err := db.Exec("INSERT INTO members VALUES ($1, $2)", number, name); err != nil {
		log.Printf("addMember: %q", err)
		return false
	}

	return true
}

func removeMember(number string) bool {
	var num_rows int
	if err := db.QueryRow("SELECT count(*) FROM members WHERE number=$1", number).Scan(&num_rows); err != nil {
		log.Printf("removeMember: %q", err)
		return false
	}
	if num_rows == 0 {
		return false
	}
	if _, err := db.Exec("DELETE FROM members WHERE number=$1", number); err != nil {
		log.Printf("removeMember: %q", err)
		return false
	}
	return true
}

func getMembers() ([]string, error) {
	rows, err := db.Query("SELECT name FROM members")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}

		members = append(members, m)
	}
	return members, nil

}

func receiveSMSHandler(w http.ResponseWriter, r *http.Request) {
	sender := r.FormValue("From")
	body := strings.ToLower(r.FormValue("Body"))

	mess := twiml.Message{
		From: twilioNumber,
		To:   sender,
	}

	if body == "lista" {
		m, err := getMembers()
		if err != nil {
			log.Printf("Error while getting members: %q", err)
			mess.Body = "Något gick fel :("
		} else {
			str := "Deltagare i jullotteriet: "
			for i := 0; i < len(m); i++ {
				str = str + m[i]
				if i < len(m)-1 {
					str = str + ", "
				}
			}
			mess.Body = str
		}

	} else if body == "avbryt" {
		mess.Body = "Du är borttagen från jullotteriet"
		ok := removeMember(sender)
		if !ok {
			mess.Body = "Kunde inte ta bort dig från jullotteriet, kanske är du inte registrerad?"
		}
	} else if body[:3] == "jul" {
		mess.Body = "Du är nu med i jullotteriet!"
		if len(body) < 5 {
			mess.Body = "Du måste ange ett namn också (skriv JUL ditt namn)"
		} else {
			ok := addMember(body[4:], sender)
			if !ok {
				mess.Body = "Kunde inte lägga till dig till jullotteriet, kanske är du redan registrerad?"
			}
		}
	}
	resp := twiml.NewResponse()
	resp.Action(mess)
	resp.Send(w)
}
