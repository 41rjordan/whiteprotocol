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

type NODE struct {
	conn chan string
	ip   string
}

var NODES_LIST []NODE
var MY_IP string
var SERVER_PORT = "4141"
var RUN_LOCAL = false

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func removeNode(nodesptr *[]NODE, ip string) {
	nodes := *nodesptr
	index := -1
	for i, node := range nodes {
		if node.ip == ip {
			index = i
			break
		}
	}
	if index == -1 {
		println("WARNING - Can't remove the node because it doesn't exist in the nodes list.")
		return
	}

	nodes[index] = nodes[len(nodes)-1]
	*nodesptr = nodes[:len(nodes)-1]
}

func getIps(nodes *[]NODE) []string {
	var ips []string
	for _, node := range *nodes {
		ips = append(ips, node.ip)
	}
	return ips
}

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
	println("white protocol by 41rjordan")

	PORT := flag.String("port", "4141", "TCP port which will be used by node")
	IS_ROOT := flag.Bool("rootnode", false, "Run node as rootnode (if true, node won't sync and will force-run on 4141 port)")
	START_NODE_PTR := flag.String("force-connect", "localhost:4141", "Node will sync with preferred node first")
	IS_LOCAL := flag.Bool("local", false, "Run node in local network (if true, node will provide local IP address to other nodes)")
	flag.Parse()

	SERVER_PORT = *PORT
	RUN_LOCAL = *IS_LOCAL

	println("Your port is: " + SERVER_PORT + ".\nI will use this port to connect to other nodes.")

	if RUN_LOCAL {
		MY_IP = fmt.Sprintf("%s", getInternalIP())
	} else {
		MY_IP = fmt.Sprintf("%s", getExternalIP())
	}
	println("Your ip is " + MY_IP + ". This ip will be used in receiving node.")

	if *IS_ROOT {
		ln, err := net.Listen("tcp", ":4141")
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		println("Listening to port 4141")
		for {
			fmt.Println("Nodes list right now:", strings.Join(getIps(&NODES_LIST), ", "))
			conn, err := ln.Accept()
			if err != nil {
				panic(err)
			}
			go handler(conn)
		}
	} else {

		START_NODE := strings.ReplaceAll(*START_NODE_PTR, "localhost", MY_IP)

		c := make(chan string)
		go connection(START_NODE, c)
		go addMe(c)
		NODES_LIST = append(NODES_LIST, NODE{c, START_NODE})

		ln, err := net.Listen("tcp", ":"+SERVER_PORT)
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		println("Listening to port " + SERVER_PORT)
		for {
			fmt.Println("Nodes list right now:", strings.Join(getIps(&NODES_LIST), ", "))
			conn, err := ln.Accept()
			if err != nil {
				panic(err)
			}
			go handler(conn)
		}
	}
}

// recursive function that gets all known nodes in network
func addMe(c chan string) {

	c <- "addme"
	resp := <-c
	msgargs := strings.Split(resp, " ")

	if msgargs[1] != "done" {
		panic("root node returned error while receiving nodes list")
	}

	ips := getIps(&NODES_LIST)

	for _, ip := range msgargs[2:] {
		if contains(ips, ip) || ip == MY_IP+":"+SERVER_PORT {
			continue
		}
		nodec := make(chan string)
		go connection(ip, nodec)
		NODES_LIST = append(NODES_LIST, NODE{nodec, ip})
		addMe(nodec)
	}
}

func connection(ip string, c chan string) {

	conn, err := net.Dial("tcp", ip)
	if err != nil {
		fmt.Println(ip, "broke the connection. Removing them from the nodes list.")
		removeNode(&NODES_LIST, ip)
		return
	}

	for {
		msg := <-c
		fmt.Fprintf(conn, MY_IP+":"+SERVER_PORT+" "+msg+"\n")

		resp, _ := bufio.NewReader(conn).ReadString(10) // 10 == \n
		c <- strings.TrimSpace(resp)
	}
}

func handler(conn net.Conn) {
	var from_ip string

	for {
		ip_with_port := MY_IP + ":" + SERVER_PORT

		// read incoming data
		buffer := make([]byte, 1024)
		_, err := conn.Read(buffer)
		if err != nil {
			if from_ip == "" {
				fmt.Println("Connection was broken without any packet exchange. Disabling connection handler.")
			} else {
				fmt.Println(from_ip, "turned down. Hope they will revive soon! Removing them from the nodes list.")
				removeNode(&NODES_LIST, from_ip)
			}
			fmt.Println("Nodes list right now: ", strings.Join(getIps(&NODES_LIST), ", "))
			return
		}

		buffer = bytes.Trim(buffer, "\x00")

		msgargs := strings.Split(strings.TrimSuffix(string(buffer), "\n"), " ")
		from_ip = msgargs[0]

		switch msgargs[1] {

		case "addme":
			strip := msgargs[0]
			fmt.Println(strip, "created first connection. Spreading their ip to nodes.")

			nodec := make(chan string)
			go connection(strip, nodec)

			ips := getIps(&NODES_LIST)

			for _, node := range NODES_LIST {
				node.conn <- "add " + strip
			}

			if !contains(ips, strip) && strip != ip_with_port {
				NODES_LIST = append(NODES_LIST, NODE{nodec, strip})
			}

			conn.Write([]byte(ip_with_port + " done " + strings.Join(getIps(&NODES_LIST), " ") + "\n"))

		case "add":
			// checking if ip already exists in nodes
			ips := getIps(&NODES_LIST)
			if contains(ips, msgargs[2]) {
				break
			}
			if msgargs[2] == ip_with_port {
				break
			}

			c := make(chan string)
			go connection(msgargs[2], c)
			NODES_LIST = append(NODES_LIST, NODE{c, msgargs[2]})

		case "whisper": // test function

			fmt.Println(msgargs[0], "whispers:", strings.Join(msgargs[2:], " "))

			for _, node := range NODES_LIST {
				node.conn <- "whisper " + strings.Join(msgargs[2:], " ")
			}

		default:
			conn.Write([]byte(ip_with_port + " err unknown_command\n"))

		}

		fmt.Println("Nodes list right now: ", strings.Join(getIps(&NODES_LIST), ", "))
	}
}
