package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
)


// validPath defines the rule that paths must conform to
var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")

type Config struct {
	dataPath string
	templates *template.Template
}

// Call contains the details of a conversation.
type Call struct {
	When 		time.Time 	`json:"when"`
	Where 		string 		`json:"where,omitempty"`
	Company 	string 		`json:"company"`
	Caller 		string 		`json:"caller"`
	Referrer 	string 		`json:"referrer"`
	Notes 		string 		`json:"notes"`
}

// save stores call data to a file as a JSON object.
func (c *Call) save() error {
	content, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename(c.Company), content, 0600)
}

func filename(company string) string {
	return config.dataPath + company + ".json"
}

// getCall fetches a call (by company name) and creates a Call object.
func getCall(company string) (*Call, error) {
	record, err := ioutil.ReadFile(filename(company))
	if err != nil {
		return nil, err
	}
	c := Call{}
	err = json.Unmarshal(record, &c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// renderTemplate parses a template file and renders it with the call data.
func renderTemplate(w http.ResponseWriter, tmpl string, c *Call) {
	err := config.templates.ExecuteTemplate(w, tmpl + ".html", c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// editHandler presents a form for entering call data.
func editHandler(w http.ResponseWriter, _ *http.Request, company string) {
	c, err := getCall(company)
	if err != nil {
		log.WithFields(log.Fields{"company": company}).Info("creating")
		c = &Call{
			Company: company,
		}
	}
	renderTemplate(w, "edit", c)
}

// saveHandler creates a Call object, stores it, and redirects to an edit view.
func saveHandler(w http.ResponseWriter, r *http.Request, company string) {
	notes := r.FormValue("notes")
	when, err := time.Parse("2006-01-02", r.FormValue("when"))
	where := r.FormValue("where")
	caller := r.FormValue("caller")
	referrer := r.FormValue("referrer")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	c := &Call{
		Company: 	company,
		When: 		when,
		Where: 		where,
		Caller:		caller,
		Referrer:	referrer,
		Notes: 		notes,
	}
	log.WithFields(log.Fields{"company": company}).Info("saving")
	err = c.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/view/" + company, http.StatusFound)
}

// viewHandler presents a read-only view of a call.
// If the call is not found, redirects to the edit view.
func viewHandler(w http.ResponseWriter, r *http.Request, company string) {
	log.WithFields(log.Fields{"company": company}).Info("viewing")
	c, err := getCall(company)
	if err != nil {
		log.WithFields(log.Fields{"company": company}).Info("does not exist")
		http.Redirect(w, r, "/edit/" + company, http.StatusFound)
		return
	}
	renderTemplate(w, "view", c)
}

// makeHandler wraps a handler with path validation and provides it with the company name.
func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func getConfig() Config {
	var err error

	get := func(key string, def string) string {
		val, ok := os.LookupEnv(key)
		if !ok {
			val = def
		}
		return val
	}

	cfg := Config{}
	cfg.dataPath = get("DATA_PATH", "data/")
	templatePath := get("TEMPLATE_PATH", "assets/")
	cfg.templates, err = template.ParseFiles(
		templatePath + "/edit.html",
		templatePath + "/view.html",
	)
	if err != nil {
		log.Fatal("unable to parse templates")
	}
	log.WithFields(log.Fields{
		"dataPath": cfg.dataPath,
		"templatePath": templatePath}).Info("configuration")
	return cfg
}

var config = getConfig()

func main() {
	http.HandleFunc("/view/", makeHandler(viewHandler))
	http.HandleFunc("/edit/", makeHandler(editHandler))
	http.HandleFunc("/save/", makeHandler(saveHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
