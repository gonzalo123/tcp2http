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
