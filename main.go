package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type application struct {
	auth struct {
		username string
		password string
	}
}

func main() {
	app := new(application)

	app.auth.username = os.Getenv("AUTH_USERNAME")
	app.auth.password = os.Getenv("AUTH_PASSWORD")

	if app.auth.username == "" {
		log.Fatal("basic auth username must be provided")
	}

	if app.auth.password == "" {
		log.Fatal("basic auth password must be provided")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.indexHandler)
	mux.HandleFunc("/csp", app.cspHandler)
	mux.HandleFunc("/other", app.otherHandler)
	mux.HandleFunc("/protected", app.basicAuth(app.protectedHandler))
	mux.HandleFunc("/slow", app.slowHandler)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	srvS := &http.Server{
		Addr:         ":8081",
		Handler:      mux,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		log.Printf("starting server on %s with self signed TLS certs", srvS.Addr)
		err := srvS.ListenAndServeTLS("./localhost.pem", "./localhost-key.pem")
		log.Fatal(err)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		log.Printf("starting server on %s with no TLS", srv.Addr)
		err := srv.ListenAndServe()
		log.Fatal(err)
	}()

	wg.Wait()
}

func (app *application) indexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `
	<!DOCTYPE html>
<html>
<head>
</head>
<body>

<table>
<tr>
<td><a href="/csp">/csp</a></td>
<td>Test CSP (look in console)</td>
</tr>
<tr>
<td><a href="/other">/other</a></td>
<td>Go here for most tests</td>
</tr>
<tr>
<td><a href="/protected">/protected</a></td>
<td>Test for basic auth</td>
</tr>
<tr>
<td><a href="/slow">/slow</a></td>
<td>You'll get a response back after 200ms</td>
</tr>
</table>

</body>
</html>`)
}

func (app *application) slowHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Millisecond * 200)
	fmt.Fprintf(w, "Sorry, that was slow")
}

func (app *application) protectedHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, admin")
}

func (app *application) cspHandler(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Add("Content-Security-Policy", "default-src https:")
	fmt.Fprintf(w, "Hello, CSP tester")
}

func (app *application) otherHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "hello-world-go",
		Value:  "this is a test cookie, yum!",
		MaxAge: 3600,
	})

	fmt.Fprintf(w, `
	<!DOCTYPE html>
<html>
<head>
<script>
function getCookies() {
	const cookiesDisplay = document.getElementById("cookies-demo");
	cookiesDisplay.textContent = "Cookies: " + document.cookie;
}

function getUserAgent() {
	const uaDisplay = document.getElementById("useragent-demo");
	uaDisplay.textContent = "Your UserAgent: " + navigator.userAgent;
}

function getTimezone() {
	const tzDisplay = document.getElementById("tz-demo");
	tzDisplay.textContent = "Timezone: " + Intl.DateTimeFormat().resolvedOptions().timeZone;
}

function networkStatus() {
	const statusDisplay = document.getElementById("network-demo");
	statusDisplay.textContent = "Network Status: " + navigator.onLine;
}

function getGeolocation() {
	navigator.geolocation.getCurrentPosition(function(position) {
		let lat = position.coords.latitude;
		let long = position.coords.longitude;

		document.getElementById("demo").innerHTML = "Lat: " + lat.toFixed(2) + " Long: " + long.toFixed(2) + "";
	});
}

function handleCheckboxClick(cb) {
	const cbDisplay = document.getElementById("cb-demo");
	if (cb.checked) {
		cbDisplay.textContent = "Thanks for checking the box"
	} else {
		cbDisplay.textContent = "You've just unchecked the box"
	}
}

function handleInputText(it) {
	const itDisplay = document.getElementById("input-text-test-confirm");
	if (it.value !== '') {
		itDisplay.textContent = "Thanks for filling in the input text field"
	} else {
		itDisplay.textContent = "You've just removed everything from the input text field"
	}
}

function inputTextOnFocus(it) {
	const itDisplay = document.getElementById("input-text-test-confirm");
	itDisplay.textContent = "focused on input text field"
}

function getLocale() {
	const userLocale =
  navigator.languages && navigator.languages.length
    ? navigator.languages[0]
    : navigator.language;

	document.getElementById("locale-demo").innerHTML = userLocale;
}

var counter = 0;
function incrementCounter() {
	const counterDisplay = document.getElementById("counter-demo");
	console.log(counter)
	counterDisplay.textContent = "Counter: " + ++counter;
	console.log(counter);
}
</script>
</head>
<body onload="getLocale(); getTimezone(); getUserAgent(); networkStatus(); getCookies();">

<table>
<tr>
<td><button type="button" onclick="getGeolocation()">Get geolocation</button></td>
<td><p id="demo">Lat: ? Long: ?</p></td>
</tr>
<tr>
<td>NA</td>
<td><p id="locale-demo">Locale: ?</p></td>
</tr>
<tr>
<td><button type="button" onclick="networkStatus()">Refresh network status</button></td>
<td><p id="network-demo">Network Status: ?</p></td>
</tr>
<tr>
<td>NA</td>
<td><p id="tz-demo">Timezone: ?</p></td>
</tr>
<tr>
<td>NA</td>
<td><p id="useragent-demo">Your UserAgent: ?</p></td>
</tr>
<tr>
<td><button type="button" onclick="getCookies()">Refresh cookies</button></td>
<td><p id="cookies-demo">Cookies: ?</p></td>
</tr>
<tr>
<td>
	<input type="checkbox" onclick='handleCheckboxClick(this);' id="cb1" name="cb1" value="Checkbox test 1">
	<label for="cb1">Checkbox test 1</label><br>
</td>
<td><p id="cb-demo">No interaction</p></td>
</tr>
<tr>
<td><button type="button" id="counter-button" onclick="incrementCounter()">Increment</button></td>
<td><p id="counter-demo">Counter: 0</p></td>
</tr>
<tr>
<td><input type="text" oninput='handleInputText(this);' onfocus='inputTextOnFocus(this);' id="input-text-test"></td>
<td><p id="input-text-test-confirm">No interaction</p></td>
</tr>
<tr>
<td><input type="text" disabled="true" id="input-text-disabled"></td>
<td>Disabled input text field</td>
</tr>
<tr>
<td><input type="text" hidden="true" id="input-text-hidden"></td>
<td>Hidden input text field</td>
</tr>
<tr>
<td><label for="cars">Choose a car:</label></td>
<td>
<select name="cars" id="cars-options" multiple>
  <option value="none">None</option>
  <option value="renault">Renault</option>
  <option value="ferrari">Ferrari</option>
  <option value="mercedes">Mercedes</option>
  <option value="porsche">Porsche</option>
  <option value="land rover">Land Rover</option>
</select>
</td>
</tr>
<tr>
<td><label for="colors">Choose a color:</label></td>
<td>
<select name="colors" id="colors-options">
  <option value="none">None</option>
  <option value="renault">Red</option>
  <option value="ferrari">Green</option>
  <option value="mercedes">Blue</option>
  <option value="porsche">Yellow</option>
  <option value="land rover">Black</option>
  <option value="land rover">White</option>
</select>
</td>
</tr>
</table>

<div id="off-screen" style="position: absolute; top: 150vh; left: 100px;">
Off page div.
</div>

</body>
</html>`)
}

func (app *application) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(app.auth.username))
			expectedPasswordHash := sha256.Sum256([]byte(app.auth.password))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
