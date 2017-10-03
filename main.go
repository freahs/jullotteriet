package main

import (
	"bitbucket.org/ckvist/twilio/twirest"
	"database/sql"
	"fmt"
	sms "github.com/freahs/jullotteri/smshandler"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"log"
	"math/rand"
	"net/http"
	"os"
)

var db *sql.DB

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s environment variable not set.", k)
	}
	return v
}

func init() {
	var err error
	if db, err = sql.Open("postgres", mustGetenv("DATABASE_URL")); err != nil {
		log.Fatalf("Error opening database: %q", err)
	}
}

func main() {
	sms.RegisterHandler("lista", listSMSHandler)
	sms.RegisterHandler("avbryt", removeSMSHandler)
	sms.RegisterHandler("jul", addSMSHandler)
	sms.RegisterHandler("starta", lotterySMSHandler)

	tclient := twirest.NewClient(
		mustGetenv("TWILIO_ACCOUNT_SID"),
		mustGetenv("TWILIO_AUTH_TOKEN"))
	tnumber := mustGetenv("TWILIO_NUMBER")

	r := mux.NewRouter()
	r.HandleFunc("/", defaultHandler)
	r.HandleFunc("/test", testHandler)
	r.HandleFunc("/sms/receive", sms.TwiloSMSHandler(tclient, tnumber))

	port := mustGetenv("PORT")
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func listSMSHandler(from, body string) string {
	m, err := getMembers()
	if err != nil {
		log.Printf("Error while getting members: %q", err)
		return "Något gick fel :("
	}
	ret := "Deltagare i jullotteriet: "
	for i := 0; i < len(m); i++ {
		ret += m[i]
		if i < len(m)-1 {
			ret += ", "
		}
	}
	return ret + "."
}

func removeSMSHandler(from, body string) string {
	if !removeMember(from) {
		return "Kunde inte ta bort dig från jullotteriet, kanske är du inte registrerad?"
	}
	return "Du är borttagen från jullotteriet"
}

func addSMSHandler(from, body string) string {
	if body == "" {
		return "Du måste ange ett namn också (skriv JUL ditt namn)"
	}
	if !addMember(from, body) {
		return "Kunde inte lägga till dig till jullotteriet, kanske är du redan registrerad?"
	}
	return "Du är nu med i jullotteriet!"
}

func lotterySMSHandler(from, body string) string {
	lottery_secret := os.Getenv("LOTTERY_SECRET")
	if lottery_secret == "" {
		return "Det finns ingen LOTTERY_SECRET, kan inte starta lotteriet."
	}
	if body == "" {
		return "Ange lösenord för att starta lotteriet."
	}
	if body == lottery_secret {
		doLottery()
		return "Lotteriet färdigt."
	}
	return "Felaktigt lösenord."
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Nothing here\n"))
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%v", "test")
}

/*
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
*/

func addMember(number, name string) bool {
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

func doLottery() {
	rows, err := db.Query("SELECT * FROM members")
	if err != nil {
		log.Printf("Could not fetch members from DB: %v", err)
	}

	numbers := make(map[string]string)
	var members []string

	defer rows.Close()
	for rows.Next() {
		var number, name string
		if err := rows.Scan(&number, &name); err != nil {
			log.Fatalf("Error while scanning row: %v", err)
		}
		numbers[name] = number
		members = append(members, name)
	}

	pairs := make(map[string]string)
	if len(members) == 1 {
		pairs[members[0]] = members[0]
	} else if len(members) > 1 {
		done := false
		var list []int
		for !done {
			list = rand.Perm(len(members))
			done = true
			for i := 0; i < len(list); i++ {
				if list[i] == i {
					done = false
				}
				pairs[members[i]] = members[list[i]]
			}
		}
	}

	for k, v := range pairs {
		sendSMS(numbers[k], "Du ska köpa en julklapp till "+v)
	}
}

func sendSMS(number, message string) {
	log.Printf("sending sms to %v, message %q", number, message)
}

/*
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
*/
