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

	"github.com/gorilla/websocket"
)

type application struct {
	auth struct {
		username string
		password string
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
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
	mux.HandleFunc("/ws/echo", app.wsEchoHandler)

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
	v := r.Header[http.CanonicalHeaderKey("x-authenticated-user")]
	if v != nil {
		fmt.Printf("x-authenticated-user header present in call to index: %s\n", v)
		w.Header().Add("x-authenticated-user", v[0])
	}

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

<br />
<h1>Websocket Test</h1>

<!-- websockets.html -->
<input id="input" type="text" />
<button onclick="send()">Send</button>
<pre id="output"></pre>
<script>
    var input = document.getElementById("input");
    var output = document.getElementById("output");
    try {
        var socket = new WebSocket("ws://localhost:8080/ws/echo");
    } catch (error) {
        console.log(error);
    }

    socket.onopen = function () {
        output.innerHTML += "Status: Connected\n";
    };

    socket.onmessage = function (e) {
        output.innerHTML += "Server: " + e.data + "\n";
    };

    function send() {
        socket.send(input.value);
        input.value = "";
    }
</script>

</body>
</html>`)
}

func (app *application) wsEchoHandler(w http.ResponseWriter, r *http.Request) {
	// There's no way for the javascript code to forward
	// any custom headers to a Update WS connection.
	// Following commented code was used to test https://github.com/grafana/xk6-browser/issues/554.
	// for k, v := range r.Header {
	// 	fmt.Println(k, v)
	// }
	// v, ok := r.Header[http.CanonicalHeaderKey("x-authenticated-user")]
	// if !ok {
	// 	fmt.Printf("x-authenticated-user header not present in call to ws/echo\n")
	// 	http.Error(w, "BadRequest", http.StatusBadRequest)
	// 	return
	// }
	// fmt.Printf("x-authenticated-user header present in call to ws/echo: %s\n", v)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("cannot update to ws connection", err)
		return
	}

	fmt.Printf("connection made with: %s\n", conn.RemoteAddr())

	for {
		// Read message from browser
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("cannot read ws message", err)
			return
		}

		// Print the message to the console
		fmt.Printf("%s sent: %s\n", conn.RemoteAddr(), string(msg))

		// Write message back to browser
		if err = conn.WriteMessage(msgType, msg); err != nil {
			log.Println("cannot write ws message", err)
			return
		}
	}
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
            const cDisplay = document.getElementById("cookies-demo");
            cDisplay.textContent = "Cookies: " + document.cookie;
        }

        function getUserAgent() {
            const uaDisplay = document.getElementById("useragent-demo");
            uaDisplay.textContent = "Your UserAgent: " + navigator.userAgent;
        }

        function getTimezone() {
            const tzDisplay = document.getElementById("timezone-demo");
            tzDisplay.textContent = "Timezone: " + Intl.DateTimeFormat().resolvedOptions().timeZone;
        }

        function networkStatus() {
            const nsDisplay = document.getElementById("network-demo");
            nsDisplay.textContent = "Network Status: " + navigator.onLine;
        }

        function getGeolocation() {
            navigator.geolocation.getCurrentPosition(function (position) {
                let lat = position.coords.latitude;
                let long = position.coords.longitude;

                document.getElementById("geolocation-demo").innerHTML = "Lat: " + lat.toFixed(2) + " Long: " + long.toFixed(2) + "";
            });
        }

        function handleCheckboxClick(cb) {
            const cbDisplay = document.getElementById("checkbox-demo");
            if (cb.checked) {
                cbDisplay.textContent = "Thanks for checking the box"
            } else {
                cbDisplay.textContent = "You've just unchecked the box"
            }
        }

        function handleInputText(it) {
            const itDisplay = document.getElementById("text-demo");
            if (it.value !== "") {
                itDisplay.textContent = "Thanks for filling in the input text field"
            } else {
                itDisplay.textContent = "You've just removed everything from the input text field"
            }
        }

        function inputTextOnFocus(it) {
            const itDisplay = document.getElementById("text-demo");
            itDisplay.textContent = "focused on input text field"
        }

        function inputTextOnFocusOut(it) {
            const itDisplay = document.getElementById("text-demo");
            itDisplay.textContent = "focused out off input text field"
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

        function selectOnChange(sel) {
            const sDisplay = document.getElementById("select-multiple-demo");
            var opts = "Selected: ", opt;
            var len = sel.options.length;
            for (var i = 0; i < len; i++) {
                opt = sel.options[i];

                if (opt.selected) {
                    opts = opts + opt.value + " ";
                }
            }
            sDisplay.textContent = opts;
        }
    </script>
</head>

<body onload="getLocale(); getTimezone(); getUserAgent(); networkStatus(); getCookies();">

    <p><a href="/">&lt; Back</a></p>

    <table>
        <tr>
            <td><button type="button" onclick="getGeolocation()">Get geolocation</button></td>
            <td>
                <p id="geolocation-demo">Lat: ? Long: ?</p>
            </td>
        </tr>
        <tr>
            <td>NA</td>
            <td>
                <p id="locale-demo">Locale: ?</p>
            </td>
        </tr>
        <tr>
            <td><button type="button" onclick="networkStatus()">Refresh network status</button></td>
            <td>
                <p id="network-demo">Network Status: ?</p>
            </td>
        </tr>
        <tr>
            <td>NA</td>
            <td>
                <p id="timezone-demo">Timezone: ?</p>
            </td>
        </tr>
        <tr>
            <td>NA</td>
            <td>
                <p id="useragent-demo">Your UserAgent: ?</p>
            </td>
        </tr>
        <tr>
            <td><button type="button" onclick="getCookies()">Refresh cookies</button></td>
            <td>
                <p id="cookies-demo">Cookies: ?</p>
            </td>
        </tr>
        <tr>
            <td>
                <input type="checkbox" onclick="handleCheckboxClick(this);" id="checkbox1" name="checkbox1"
                    value="Checkbox test 1">
                <label for="checkbox1">Checkbox test 1</label><br>
            </td>
            <td>
                <p id="checkbox-demo">No interaction</p>
            </td>
        </tr>
        <tr>
            <td><button type="button" id="counter-button" onclick="incrementCounter()">Increment</button></td>
            <td>
                <p id="counter-demo">Counter: 0</p>
            </td>
        </tr>
        <tr>
            <td><input type="text" oninput="handleInputText(this);" onfocus="inputTextOnFocus(this);" onfocusout="inputTextOnFocusOut(this);" id="text1"></td>
            <td>
                <p id="text-demo">No interaction</p>
            </td>
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
            <td><label for="numbers">Choose one or more numbers:</label></td>
            <td>
                <select name="numbers" id="numbers-options" onchange="selectOnChange(this)" multiple>
                    <option value="zero">Zero</option>
                    <option value="one">One</option>
                    <option value="two">Two</option>
                    <option value="three">Three</option>
                    <option value="four">Four</option>
                    <option value="five">Five</option>
                </select>
                <p id="select-multiple-demo">Nothing selected</p>
            </td>
        </tr>
        <tr>
            <td><label for="colors">Choose a color:</label></td>
            <td>
                <select name="colors" id="colors-options">
                    <option value="none">None</option>
                    <option value="red">Red</option>
                    <option value="green">Green</option>
                    <option value="blue">Blue</option>
                    <option value="yellow">Yellow</option>
                    <option value="black">Black</option>
                    <option value="white">White</option>
                </select>
            </td>
        </tr>
    </table>

    <div id="off-screen" style="position: absolute; top: 150vh; left: 100px;">
        Off page div
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
