## Transforming TCP sockets to HTTP with Go

Sometimes we need to work with legacy applications. Legacy application that are hard to rewrite and hard to change. Imagine, for example, this application is sending raw TCP sockets to communicate with another process. Raw TCP sockets are fast but they have various problems, for example all data is sent in plain text over the network and without authentication (if we don't implement one protocol).

One solution is use https connections instead. We can also authenticate those requests with an Authentication Bearer. For example I've created one simple http server with Python and Flask:

```python
import logging
import os
from functools import wraps

from flask import Flask, request, abort
from flask import jsonify

logging.basicConfig(level=logging.DEBUG)

logger = logging.getLogger(__name__)
app = Flask(__name__)


def authorize_bearer(bearer):
    def authorize(f):
        @wraps(f)
        def decorated_function(*args, **kws):
            if 'Authorization' not in request.headers:
                abort(401)

            data = request.headers['Authorization']

            if str.replace(str(data), 'Bearer ', '') != bearer:
                abort(401)

            return f(*args, **kws)

        return decorated_function

    return authorize


@app.route('/register', methods=['POST'])
@authorize_bearer(bearer=os.getenv('TOKEN'))
def hello_world():
    req_data = request.get_json()
    logger.info(req_data)
    return jsonify({"status": "OK", "request_data": req_data})
```

Now we only need to change our legacy application to use one http client instead raw TCP sockets. But sometimes it's not possible. Imagine, for example, if this application runs on a old OS without https support or we cannot find and compile an http client in the legacy application.

One possible solution is isolate the application and change only the destination of the TCP socket. Instead the original ip address whe can use localhost and we can create a proxy at localhost that listen to TCP sockets and send the information to the HTTP server. 

We're going to build this proxy in Go. We can do it with any language (Python, C#, Javascript, ...). My Kung Fu in Go is not so good (I'm more comfortable with Python) but it's not so difficult and we can build a binary with our proxy for Windows, Linux and Mac without any problem. Then we only need to copy the binary into the target host and it works (no installation, no SDK, nothing. Just copy and run)

```go
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

func main() {
	port, closeConnection, url := parseFlags()
	openSocket(*port, *closeConnection, *url, onMessage)
}

func onMessage(url string, buffer string) {
	bearer := os.Getenv("TOKEN")
	client := &http.Client{}
	req, _ := http.NewRequest("POST", url, strings.NewReader(buffer))
	req.Header.Add("Authorization", "Bearer "+bearer)
	req.Header.Add("content-type", "application/json")
	resp, err := client.Do(req)

	if err != nil {
		log.Println(err)
	} else {
		if resp.Status == "200" {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			log.Println(result["status"])
		} else {
			log.Println("Response status: " + resp.Status)
		}
		defer resp.Body.Close()
	}
}

func parseFlags() (*string, *bool, *string) {
	port := flag.String("port", "7777", "port number")
	closeConnection := flag.Bool("close", true, "Close connection")
	url := flag.String("url", "http://localhost:5000/register", "Destination endpoint")
	flag.Parse()
	return port, closeConnection, url
}

func openSocket(port string, closeConnection bool, url string, onMessage func(url string, buffer string)) {
	PORT := "localhost:" + port
	l, err := net.Listen("tcp4", PORT)
	log.Printf("Serving %s\n", l.Addr().String())
	if err != nil {
		log.Fatalln(err)
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatalln(err)
		}
		go handleConnection(c, closeConnection, url, onMessage)
	}
}

func handleConnection(c net.Conn, closeConnection bool, url string, onMessage func(url string, buffer string)) {
	log.Printf("Accepted connection from %s\n", c.RemoteAddr().String())
	for {
		ip, port, err := net.SplitHostPort(c.RemoteAddr().String())
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			log.Println(err)
		}

		message := map[string]interface{}{
			"body":   strings.TrimSpace(netData),
			"ipFrom": ip,
			"port":   port,
		}

		log.Printf("Making request with %s\n", message)
		bytesRepresentation, err := json.Marshal(message)
		if err != nil {
			log.Println(err)
		} else {
			//buffer := bytes.NewBuffer(bytesRepresentation)
			onMessage(url, string(bytesRepresentation))
		}

		if closeConnection {
			c.Close()
			return
		}
	}
	c.Close()
}
```

And that's all. We can upgrade our legacy application without almost changing the code.