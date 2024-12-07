package main

import (
	// "fmt"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

var (
	maxAttempts      = 100
	Assignments      map[string]string
	participantsPath = "./participants.json"
	assignmentsPath  = "./assignments.base64"
	cookieExpires    = time.Date(time.Now().Year(), time.December, 31, 23, 59, 59, 0, time.Local)
)

// GetAssignments either reads the assignments from the assignments file or creates new assignments based on the
// participants file.
func GetAssignments() map[string]string {
	f, err := os.Open(participantsPath)
	if err != nil {
		log.Fatalf("error reading participants file: %v", err)
	}
	defer f.Close()

	var participants map[string][]string
	if err = json.NewDecoder(f).Decode(&participants); err != nil {
		log.Fatalf("invalid participants file: %v", err)
	}

	assignments, err := ReadAssignments(assignmentsPath)
	if os.IsNotExist(err) {
		log.Printf("no assignments file found, generating new assignments...")
		assignments, err = CreateAssignments(participants)
		if err == nil {
			log.Printf("writing assignments...")
			err = WriteAssignments(assignmentsPath, assignments)
		}
	}

	if err != nil {
		log.Fatal(err)
	}

	return assignments
}

// ReadAssignments reads the base64-encoded assignments from the given file path. Assignments are base64-encoded to
// prevent accidental or intentional peeking.
func ReadAssignments(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	encoded, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading assignments file: %v", err)
	}

	data, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("error decoding assignments data: %v", err)
	}

	var assignments map[string]string
	err = json.Unmarshal(data, &assignments)
	if err != nil {
		return nil, fmt.Errorf("invalid assignments data: %v", err)
	}

	return assignments, nil
}

// WriteAssignments writes the given assignments to the given file path. Assignments are base64-encoded to prevent
// accidental or intentional peeking.
func WriteAssignments(path string, assignments map[string]string) error {
	if assignments == nil {
		return fmt.Errorf("no assignments to write")
	}

	data, err := json.Marshal(assignments)
	if err != nil {
		return fmt.Errorf("error marshalling assignments: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	if err := os.WriteFile(path, []byte(encoded), 0666); err != nil {
		return fmt.Errorf("error writing assignments file: %v", err)
	}

	return nil
}

// CreateAssignments creates a secret Santa assignment for the given participants. Each participant is assigned another
// participant to give a gift to. The assignments are randomized and checked against the constraints given in the
// participants map. If no valid assignment can be found after a certain number of attempts, an error is returned.
//
// Not very efficient, but good enough for small numbers of participants.
func CreateAssignments(participants map[string][]string) (map[string]string, error) {
	if len(participants) < 2 {
		return nil, fmt.Errorf("not enough participants")
	}

	names := make([]string, 0, len(participants))
	disallowed := make(map[string]map[string]bool)

	for n1, forbidden := range participants {
		names = append(names, n1)
		for _, n2 := range forbidden {
			if _, ok := participants[n2]; !ok {
				return nil, fmt.Errorf("invalid constraint \"%v\" for \"%v\"", n2, n1)
			}
			if disallowed[n1] == nil {
				disallowed[n1] = make(map[string]bool)
			}
			disallowed[n1][n2] = true
		}
	}

	for i := 0; i < maxAttempts; i++ {
		sort.Strings(names)
		rand.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
		res := make(map[string]string)
		invalid := false

		for i := 0; !invalid && i < len(names); i++ {
			n1 := names[i]
			n2 := names[(i+1)%len(names)]
			res[n1] = n2
			invalid = disallowed[n1][n2]
		}

		if !invalid {
			return res, nil
		}
	}

	return nil, fmt.Errorf("no valid assignment found")
}

func GetIp(r *http.Request) string {
	if fwd := r.Header.Get("x-forwarded-for"); fwd != "" {
		ips := strings.SplitN(fwd, ",", 2)
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	addr := r.RemoteAddr
	ip := strings.SplitN(addr, ":", 2)[0]
	return strings.TrimSpace(ip)
}

// GetClaimedName returns the claimed name from the cookie, or an empty string if no name is claimed.
func GetClaimedName(r *http.Request) string {
	cookie, err := r.Cookie("name")

	switch err {
	case nil:
		return cookie.Value
	case http.ErrNoCookie:
		return ""
	default:
		panic(err) // not possible but required
	}
}

// SetClaimedName stores the claimed named in a cookie.
func SetClaimedName(name string, w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    "name",
		Value:   name,
		Path:    "/",
		Expires: cookieExpires,
	})
}

// DeleteClaimedName deletes the cookie with the claimed name.
func DeleteClaimedName(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("name")
	if err != nil {
		panic(err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "name",
		Value:   "",
		Path:    "/",
		Expires: cookie.Expires,
		MaxAge:  -1,
	})
}

func ExecuteTemplate(w http.ResponseWriter, name string, data interface{}) {
	t, err := template.ParseFiles("templates/page.gohtml")
	if err != nil {
		panic(err)
	}
	t, err = t.Parse("{{template \"head\" .}}{{template \"" + name + "\" .}}{{template \"foot\" .}}")
	if err != nil {
		panic(err)
	}

	err = t.Execute(w, data)
	if err != nil {
		panic(err)
	}
}

func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if x := recover(); x != nil {
				log.Println(x)
				s := http.StatusInternalServerError
				http.Error(w, http.StatusText(s), s)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func LogHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetIp(r)
		name := GetClaimedName(r)
		if name == "" {
			name = "none"
		}
		log.Printf("%v ip=%v, name=%v", r.URL, ip, name)
		next.ServeHTTP(w, r)
	})
}

func ResetHandler(w http.ResponseWriter, r *http.Request) {
	nameInUrl := mux.Vars(r)["name"]
	claimedName := GetClaimedName(r)

	if _, ok := Assignments[nameInUrl]; !ok {
		http.NotFound(w, r)
	} else if claimedName == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	} else if claimedName != nameInUrl {
		ExecuteTemplate(w, "wrong-name", map[string]interface{}{"Name": claimedName})
	} else {
		DeleteClaimedName(w, r)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
	}
}

func ClaimNameHandler(w http.ResponseWriter, r *http.Request) {
	nameInUrl := mux.Vars(r)["name"]
	claimedName := GetClaimedName(r)

	if _, ok := Assignments[nameInUrl]; !ok {
		http.NotFound(w, r)
	} else if claimedName == "" {
		ExecuteTemplate(w, "name", map[string]interface{}{"Name": nameInUrl})
	} else if claimedName != nameInUrl {
		ExecuteTemplate(w, "wrong-name", map[string]interface{}{"Name": claimedName})
	} else {
		http.Redirect(w, r, "/"+claimedName+"/reveal", http.StatusTemporaryRedirect)
	}
}

func RevealHandler(w http.ResponseWriter, r *http.Request) {
	nameInUrl := mux.Vars(r)["name"]
	claimedName := GetClaimedName(r)

	if _, ok := Assignments[nameInUrl]; !ok {
		http.NotFound(w, r)
	} else if claimedName == "" {
		SetClaimedName(nameInUrl, w)
		http.Redirect(w, r, "/"+nameInUrl+"/reveal", http.StatusTemporaryRedirect)
	} else if claimedName != nameInUrl {
		ExecuteTemplate(w, "wrong-name", map[string]interface{}{"Name": claimedName})
	} else {
		ExecuteTemplate(w, "reveal", map[string]interface{}{"Name": claimedName, "Assigned": Assignments})
	}
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	claimedName := GetClaimedName(r)
	if claimedName != "" {
		http.Redirect(w, r, "/"+claimedName, http.StatusTemporaryRedirect)
	} else {
		ExecuteTemplate(w, "root", map[string]interface{}{"Assigned": Assignments})
	}
}

func main() {

	Assignments = GetAssignments()

	router := mux.NewRouter()
	router.Use(ErrorHandler, LogHandler)
	router.HandleFunc("/{name}/reset", ResetHandler)
	router.HandleFunc("/{name}/reveal", RevealHandler)
	router.HandleFunc("/{name}", ClaimNameHandler)
	router.HandleFunc("/", RootHandler)

	server := &http.Server{Handler: router}

	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}

	log.Printf("starting listener on port... %v", port)
	l, err := net.Listen("tcp4", ":"+port)
	if err != nil {
		panic(err)
	}

	log.Printf("server started")
	err = server.Serve(l)
	log.Printf("server stopped: %v", err)
}
