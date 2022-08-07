package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

func getExternalIP() string {
	req, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		panic(err)
	}
	defer req.Body.Close()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	var ip struct {
		Query string
	}
	json.Unmarshal(body, &ip)

	return ip.Query
}

func getInternalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return fmt.Sprintf("%s", localAddr.IP)
}

func main() {
	fmt.Print("\033[H\033[2J")
	println("White Protocol v" + VERSION + " by 41rjordan")
	println(USER_AGENT)

	PORT := flag.String("port", "4141", "TCP port which will be used by node")
	MAX_NODES_PTR := flag.Int("max-nodes", 10, "Maximum amount of nodes to be connected with")
	IS_ROOT := flag.Bool("rootnode", false, "Run node as rootnode (if true, node won't sync)")
	START_NODE_PTR := flag.String("force-connect", "localhost:4141", "Node will sync with preferred node first")
	IS_LOCAL := flag.Bool("local", false, "Run node in local network (if true, node will provide local IP address to other nodes)")
	DM_PASSWORD_PTR := flag.String("dm-password", "1234", "'print' user command password")
	flag.Parse()

	SERVER_PORT = *PORT
	RUN_LOCAL = *IS_LOCAL
	MAX_NODES = *MAX_NODES_PTR
	DM_PASSWORD = *DM_PASSWORD_PTR

	println("Your port is: " + SERVER_PORT + ".\nI will use this port to connect to other nodes.")

	if RUN_LOCAL {
		MY_IP = getInternalIP()
	} else {
		MY_IP = getExternalIP()
	}
	
	println("Your ip is " + MY_IP + ". This ip will be used in receiving node.")

	START_NODE := strings.ReplaceAll(*START_NODE_PTR, "localhost", MY_IP)
	MY_IP = MY_IP + ":" + SERVER_PORT

	ln, err := net.Listen("tcp", ":"+SERVER_PORT)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	if !fileExists("keypair") {
		fmt.Println("Seems it's the first run! Creating new P2P keypair.")
		createMessageKeypair()
	}

	println("Listening to port " + SERVER_PORT)
	for {
		if !*IS_ROOT && len(NODES_LIST) == 0 {
			println("No nodes connected, trying to connect to the preferred node...")

			c := make(chan []byte)
			go connection(START_NODE, c, false)
			go addMe(c)
			addNode(&NODES_LIST, c, START_NODE)

			fmt.Println("CONNECTION STATUS --", len(NODES_LIST), "connected (", MAX_NODES, "limit )")
		}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go handler(conn)
	}
}

// recursive function that gets all known nodes in network
func addMe(c chan []byte) {

	c <- buildMessage(MY_IP, USER_AGENT, "spreadme", []string{})
	resp := <-c
	msg, err := deserializeResponse(resp[32:])
	if err != nil {
		return;
	}

	if msg.Response != errorCodes["done"] {
		return;
	}
	if msg.ResponseData == "" {
		return;
	}

	ips := getIps(&NODES_LIST)

	for _, ip := range strings.Split(msg.ResponseData, " ") {
		if contains(ips, ip) || ip == MY_IP {
			continue
		}
		nodec := make(chan []byte)
		go connection(ip, nodec, false)
		addNode(&NODES_LIST, nodec, ip)
		addMe(nodec)
	}
}

func connection(ip string, c chan []byte, verifyConnection bool) { // if verifyConnection is true, function will wait for public key to verify with

	var publicKey []byte
	if verifyConnection {
		publicKey = <-c
	}

	conn, err := net.Dial("tcp", ip)
	if err != nil {
		fmt.Println(ip, "broke the connection. Removing them from the nodes list.")
		removeNode(&NODES_LIST, ip)
		close(c)
		return
	}

	connect := buildMessage(MY_IP, USER_AGENT, "connect", []string{})
	fmt.Fprintf(conn, string(connect))
	if verifyConnection {
		resp, _ := bufio.NewReader(conn).ReadBytes(10)

		if !verifyMessageSignature(resp, publicKey) { // currently unused
			fmt.Println("[WARNING] Message from", ip, "has invalid signature. Most likely, either you or they were MITMed.\n[WARNING] Removing them from the nodes list. If this warning will appear again with different node,\n[WARNING] scan yourself with an antivirus.")
			removeNode(&NODES_LIST, ip)
			close(c)
			return
		}
	}

	for {
		msg := <-c
		fmt.Fprintf(conn, string(msg))

		resp, _ := bufio.NewReader(conn).ReadBytes(10) // 10 == \n
		c <- resp
	}
}

func handler(conn net.Conn) {
	if len(NODES_LIST) >= MAX_NODES {
		fmt.Println("Nodes limit reached. Cancelling request.")
		return
	}

	var from_ip string

	for {

		// read incoming data
		buffer := make([]byte, 65536)
		_, err := conn.Read(buffer)
		if err != nil {
			if from_ip == "" {
				fmt.Println("Connection was broken without any packet exchange. Disabling connection handler.")

			} else if isAuthenticated(&NODES_LIST, from_ip) {
				removeNode(&NODES_LIST, from_ip)
			}
			fmt.Println("CONNECTION STATUS --", len(NODES_LIST), "connected (", MAX_NODES, "limit )")
			return
		}

		buffer = bytes.Trim(buffer, "\x00")

		for _, msg := range strings.Split(string(buffer), "\n") {

			if msg == "" {
				continue
			}

			message, err := deserializeMessage(msg)
			if err != nil {
				response := buildResponse(MY_IP, USER_AGENT, "invalid_request", "")
				conn.Write(response)
				continue
			}

			ips := getIps(&NODES_LIST)
			from_ip = message.FromIP

			switch message.Task {

			case "connect": // actually a nop command, nodes are sending it to notify a recipient about their receiving ip
				
				keypair, err := getMessageKeypair()
				if err != nil {
					panic(err)
				}
				response := buildResponse(MY_IP, USER_AGENT, "done", keypair.PublicKey)
				conn.Write(response)

			case "spreadme":

				nodec := make(chan []byte)
				go connection(message.FromIP, nodec, false)

				notifyNodes(&NODES_LIST, message.FromIP)

				if !contains(ips, message.FromIP) && message.FromIP != MY_IP {
					addNode(&NODES_LIST, nodec, message.FromIP)
				}

				response := buildResponse(MY_IP, USER_AGENT, "done", strings.Join(ips, " "))
				conn.Write(response)
			
			case "addme":

				// checking if ip already exists in nodes
				if contains(ips, message.FromIP) {
					break
				}
				if message.FromIP == MY_IP {
					break
				}

				c := make(chan []byte)
				go connection(message.FromIP, c, false)
				c <- buildMessage(MY_IP, USER_AGENT, "addme", []string{})

				addNode(&NODES_LIST, c, message.FromIP)

				response := buildResponse(MY_IP, USER_AGENT, "done", "")
				conn.Write(response)

			case "add":
				if !isAuthenticated(&NODES_LIST, message.FromIP) {
					response := buildResponse(MY_IP, USER_AGENT, "unauthorized", "")
					conn.Write(response)
					break
				}

				// checking if ip already exists in nodes
				if contains(ips, message.TaskArgs[0]) {
					break
				}
				if message.TaskArgs[0] == MY_IP {
					break
				}

				c := make(chan []byte)
				go connection(message.TaskArgs[0], c, false)
				c <- buildMessage(MY_IP, USER_AGENT, "addme", []string{})

				addNode(&NODES_LIST, c, message.TaskArgs[0])

				response := buildResponse(MY_IP, USER_AGENT, "done", "")
				conn.Write(response)
			
			case "print": // test command, based on this thing you can create an RPC or something

				if message.TaskArgs[0] != DM_PASSWORD {
					response := buildResponse(MY_IP, USER_AGENT, "wrong_password", "")
					conn.Write(response)
					break
				}

				fmt.Println(message.FromIP, "whispers via 'print' command:", strings.Join(message.TaskArgs[1:], " "))

				response := buildResponse(MY_IP, USER_AGENT, "done", "")
				conn.Write(response)

			default:
				response := buildResponse(MY_IP, USER_AGENT, "unknown_command", "")
				conn.Write(response)

			}

			fmt.Println("CONNECTION STATUS --", len(NODES_LIST), "connected (", MAX_NODES, "limit )")
		}
	}
}
