package main

import (
	"net"
	"fmt"
	"strings"
	"bufio"
	"bytes"
	"flag"
)

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
	index := -1;
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

type NODE struct {
	conn chan string
	ip string
}

var NODES_LIST []NODE
var SERVER_PORT = "4141"

func getIps(nodes *[]NODE) []string {
	var ips []string
	for _, node := range *nodes {
		ips = append(ips, node.ip)
	}
	return ips
}

func GetOutboundIP() net.IP {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    localAddr := conn.LocalAddr().(*net.UDPAddr)

    return localAddr.IP
}

func main() {
	println("white network by 41rjordan")

	PORT := flag.String("port", "4141", "TCP port which will be used by node")
	IS_ROOT := flag.Bool("rootnode", false, "Run node as rootnode (if true, node won't sync and will force-run on 4141 port)")
	START_NODE_PTR := flag.String("force-connect", "localhost:4141", "Node will sync with preferred node first")
	flag.Parse()

	SERVER_PORT = *PORT
	
	println("Your port is: " + SERVER_PORT + ".\nI will use this port to connect to other nodes.")

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
		myip := fmt.Sprintf("%s", GetOutboundIP())
		println("Your ip is " + myip + ". This ip will be used in receiving node.")
		START_NODE := strings.ReplaceAll(*START_NODE_PTR, "localhost", myip)

		c := make(chan string)
		go connection(START_NODE, c)
		go addMe(c)
		NODES_LIST = append(NODES_LIST, NODE{c, START_NODE})
		
		ln, err := net.Listen("tcp", ":" + SERVER_PORT)
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
	myip := fmt.Sprintf("%s", GetOutboundIP()) + ":" + SERVER_PORT

	c <- "addme"
	resp := <- c
	msgargs := strings.Split(resp, " ")

	if msgargs[1] != "done" {
		panic("root node returned error while receiving nodes list")
	}

	ips := getIps(&NODES_LIST)

	for _, ip := range msgargs[2:] {
		if contains(ips, ip) || ip == myip {
			continue
		}
		nodec := make(chan string)
		go connection(ip, nodec)
		NODES_LIST = append(NODES_LIST, NODE{nodec, ip})
		addMe(nodec)
	}
}

func connection(ip string, c chan string) {
	myip := fmt.Sprintf("%s", GetOutboundIP()) + ":" + SERVER_PORT

	conn, err := net.Dial("tcp", ip)
	if err != nil {
		fmt.Println(ip, "broke the connection. Removing them from the nodes list.")
		removeNode(&NODES_LIST, ip)
		return
	}

	for {
		msg := <- c
		fmt.Fprintf(conn, myip + " " + msg + "\n")

		resp, _ := bufio.NewReader(conn).ReadString(10) // 10 == \n
		c <- strings.TrimSpace(resp)
	}
}

func handler(conn net.Conn) {
	var from_ip string

	for {
		myip := fmt.Sprintf("%s", GetOutboundIP()) + ":" + SERVER_PORT

    	// store incoming data
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

			if !contains(ips, strip) && strip != myip {
				NODES_LIST = append(NODES_LIST, NODE{nodec, strip})
			}

			conn.Write([]byte(myip + " done " + strings.Join(getIps(&NODES_LIST), " ") + "\n"))

		case "add":
			// checking if ip already exists in nodes
			ips := getIps(&NODES_LIST)
			if contains(ips, msgargs[2]) {
				break
			}
			if msgargs[2] == myip {
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
			conn.Write([]byte(myip + " err unknown_command\n"))

		}

		fmt.Println("Nodes list right now: ", strings.Join(getIps(&NODES_LIST), ", "))
	}
}