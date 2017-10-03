package main

import (
	"bitbucket.org/ckvist/twilio/twirest"
	"database/sql"
	sms "github.com/freahs/jullotteri/smshandler"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"log"
	"math/rand"
	"net/http"
	"os"
)

var db *sql.DB
var tclient *twirest.TwilioClient
var tnumber string

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

	tclient = twirest.NewClient(
		mustGetenv("TWILIO_ACCOUNT_SID"),
		mustGetenv("TWILIO_AUTH_TOKEN"))
	tnumber = mustGetenv("TWILIO_NUMBER")
}

func main() {
	sms.RegisterHandler("lista", listSMSHandler)
	sms.RegisterHandler("avbryt", removeSMSHandler)
	sms.RegisterHandler("jul", addSMSHandler)
	sms.RegisterHandler("starta", lotterySMSHandler)

	r := mux.NewRouter()
	r.HandleFunc("/", defaultHandler)
	r.HandleFunc("/sms/receive", sms.TwiloSMSHandler(tclient, tnumber))

	port := mustGetenv("PORT")
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func defaultHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Nothing here\n"))
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
	if _, err := tclient.Request(twirest.SendMessage{
		Text: message,
		From: tnumber,
		To:   number,
	}); err != nil {
		log.Printf("Could not send SMS: %v", err)
		return
	}
}
