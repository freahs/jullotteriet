package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	cookieExpires time.Time
	Assigned      = make(map[string]string)
)

func usage() {
	Name := os.Args[0]
	fmt.Println(
		`USAGE: `+Name+` [OPTIONS] NAMES...\n`,
		`NAMES are the names of the participants\n\n`,
		`  --file FILE           The path to a file with constraints for the matching`,
		`  --seed INT            The initial seed`,
		`  --max_attempts INT    Maximum number of seeds to try before giving up`,
	)
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

func GetCookie(name string, r *http.Request) string {
	cookie, err := r.Cookie(name)

	switch err {
	case nil:
		return cookie.Value
	case http.ErrNoCookie:
		return ""
	default:
		panic(err)
	}
}

func SetCookie(name, value string, w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:    name,
		Value:   value,
		Path:    "/",
		Expires: cookieExpires,
	})
}

func DeleteCookie(name string, w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(name)
	if err != nil {
		panic(err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:    name,
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

func ResetHandler(w http.ResponseWriter, r *http.Request) {
	if claim := GetCookie("claim", r); claim == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	DeleteCookie("claim", w, r)
	http.Redirect(w, r, "/", http.StatusPermanentRedirect)
}

func NameHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	claim := GetCookie("claim", r)
	ip := GetIp(r)

	if claim == "" {
		claim = "none"
	} else if claim != name {
		ExecuteTemplate(w, "wrong-name", map[string]interface{}{"Name": name})
		log.Printf("%v %v claim=%v", ip, name, claim)
		return
	}

	ExecuteTemplate(w, "name", map[string]interface{}{"Name": name})
	log.Printf("%v %v claim=%v", ip, name, claim)
}

func RevealHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	claim := GetCookie("claim", r)
	ip := GetIp(r)

	if claim == "" {
		SetCookie("claim", name, w)
		http.Redirect(w, r, "/"+name+"/reveal", http.StatusPermanentRedirect)
	} else if claim == name {
		ExecuteTemplate(w, "reveal", map[string]interface{}{"Name": name, "Assigned": Assigned})
		log.Printf("%v %v claim=%v", ip, name, claim)
	}
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	if name := GetCookie("claim", r); name != "" {
		http.Redirect(w, r, "/"+name, http.StatusPermanentRedirect)
		return
	}
	ExecuteTemplate(w, "root", map[string]interface{}{"Assigned": Assigned})
}

func init() {
	year := time.Now().Year()
	month := int(time.December)
	day := 25

	err := func() error {
		val, ok := os.LookupEnv("COOKIE_EXPIRES")
		if !ok {
			return nil
		}
		re := regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})$`)
		res := re.FindStringSubmatch(val)
		if len(res) != 4 {
			return fmt.Errorf("invalid format string %v", val)
		}
		var err error
		if year, err = strconv.Atoi(res[1]); err != nil {
			return fmt.Errorf("invalid year format: %v", res[1])
		}
		if month, err = strconv.Atoi(res[2]); err != nil {
			return fmt.Errorf("invalid month format: %v", res[2])
		}
		if day, err = strconv.Atoi(res[3]); err != nil {
			return fmt.Errorf("invalid day format: %v", res[3])
		}
		return nil
	}()

	if err != nil {
		log.Fatalf("error parsing COOKIE_EXPIRES: %v", err)
	}

	cookieExpires = time.Date(year, time.December, 25, 0, 0, 0, 0, time.Local)

}

func main() {

	defer func() {
		if r := recover(); r != nil {
			log.Printf("error: %v", r)
			usage()
		}
	}()

	args, err := ParseArgs(os.Args[1:])
	if err != nil {
		panic(err)
	}

	log.Printf("starting with the following participants")
	for name, constraints := range args.Constraints {
		if len(constraints) == 0 {
			log.Printf("  •%v", name)
		} else {
			log.Printf("  •%v (will not be assigned to %v)", name, strings.Join(constraints, ", "))
		}
	}

	assigned, err := args.Assign()
	if err != nil {
		panic(err)
	}

	if len(assigned) == 0 {
		panic(fmt.Errorf("no participants"))
	}

	router := mux.NewRouter()
	router.Use(ErrorHandler)
	router.HandleFunc("/{name}/reset", ResetHandler)
	router.HandleFunc("/{name}/reveal", RevealHandler)
	router.HandleFunc("/{name}", NameHandler)
	router.HandleFunc("/", RootHandler)

	server := &http.Server{Handler: router}
	l, err := net.Listen("tcp4", ":8081")
	if err != nil {
		panic(err)
	}

	err = server.Serve(l)
	log.Printf("server stopped: %v", err)
}
