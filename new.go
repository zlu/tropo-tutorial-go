package main

import (
    "http"
    "io/ioutil"
    "os"
    "regexp"
    "template"
    "io"
    "net"
    "bufio"
    "strconv"
    "strings"
    "log"
)

type page struct {
    title string
    to    string
    body  []byte
}

type readClose struct {
	io.Reader;
	io.Closer;
}

type badStringError struct {
	what	string;
	str	string;
}

const (
	tropoURL = "http://api.tropo.com/1.0/sessions?action=create"
	tropoToken = "1aba4b151514ae4caaf8340879b3e456893a5f5f7d13be43d5df9546c147090c4773a68016b0dae4da7d66bc"
)

func (p *page) save() os.Error {
    filename := p.title + ".txt"
	sendsms(p.to, string(p.body))
    return ioutil.WriteFile(filename, p.body, 0600)
}

func sendsms(number, msg string) (err os.Error) {
	log.Println("sending sms from " + number + " with the following message: " + msg)
    var r *http.Response
	url, _ := http.ParseURL(tropoURL + "&to=" + number + "&msg=" + http.URLEscape(msg) + "&token=" + tropoToken)
	log.Println("url: " + url.Raw)
    r, err = Get(url)

	if err != nil {
		return
	}

	if r.StatusCode != 200 {
		err = os.ErrorString("Tropo returned: " + r.Status)
		return
	}

	log.Println("tropo returned: " + r.Status)
	var buf []byte
	io.ReadFull(r.Body, buf[:])
	log.Println(string(buf))
	r.Body.Close()
	return
}

func Get(url *http.URL) (r *http.Response, err os.Error) {
	var req http.Request;

	req.URL = url

	if r, err = send(&req); err != nil {
		return
	}
	return
}

// Given a string of the form "host", "host:port", or "[ipv6::address]:port",
// return true if the string includes a port.
func hasPort(s string) bool { 
	return strings.LastIndex(s, ":") > strings.LastIndex(s, "]")
}

func send(req *http.Request) (resp *http.Response, err os.Error) {
	addr := req.URL.Host
	if !hasPort(addr) {
		addr += ":http"
	}
	conn, err := net.Dial("tcp", "", addr)
	if err != nil {
		return nil, err
	}

	err = req.Write(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	reader := bufio.NewReader(conn)
	resp, err = http.ReadResponse(reader, "GET")
	if err != nil {
		conn.Close()
		return nil, err
	}

	r := io.Reader(reader)
	if v := resp.GetHeader("Content-Length"); v != "" {
		n, err := strconv.Atoi64(v)
		if err != nil {
//			return nil, &badStringError{"invalid Content-Length", v}
		}
		r = io.LimitReader(r, n)
	}
	resp.Body = readClose{r, conn}

	return
}

func loadPage(title string) (*page, os.Error) {
    filename := title + ".txt"
    body, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    return &page{title: title, body: body}, nil
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
    p, err := loadPage(title)
    if err != nil {
        http.Redirect(w, r, "/edit/"+title, http.StatusFound)
        return
    }
    renderTemplate(w, "view", p)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
    p, err := loadPage(title)
    if err != nil {
        p = &page{title: title}
    }
    renderTemplate(w, "edit", p)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
    to := r.FormValue("to")
    body := r.FormValue("body")
	p := &page{title: title, to: to, body: []byte(body)}
    err := p.save()
    if err != nil {
        http.Error(w, err.String(), http.StatusInternalServerError)
        return
    }
    http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

var templates = make(map[string]*template.Template)

func init() {
    for _, tmpl := range []string{"edit", "view"} {
        templates[tmpl] = template.MustParseFile(tmpl+".html", nil)
    }
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *page) {
    err := templates[tmpl].Execute(p, w)
    if err != nil {
        http.Error(w, err.String(), http.StatusInternalServerError)
    }
}

const lenPath = len("/view/")

var titleValidator = regexp.MustCompile("^[a-zA-Z0-9]+$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        title := r.URL.Path[lenPath:]
        if !titleValidator.MatchString(title) {
            http.NotFound(w, r)
            return
        }
        fn(w, r, title)
    }
}

func main() {
    http.HandleFunc("/view/", makeHandler(viewHandler))
    http.HandleFunc("/edit/", makeHandler(editHandler))
    http.HandleFunc("/save/", makeHandler(saveHandler))
    http.ListenAndServe(":8080", nil)
}