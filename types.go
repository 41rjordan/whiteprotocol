package main

import (
	"sync"
)

// Response error codes
var errorTexts = map[int]string{
	0: "done",
	1: "unauthorized",
	2: "wrong_password",
	3: "invalid_request",
}

var errorCodes = map[string]int{
	"done": 0,
	"unauthorized": 1,
	"wrong_password": 2,
	"invalid_request": 3,
}

var (
	NODES_LIST []NODE
	MY_IP string
	SERVER_PORT string
	RUN_LOCAL bool
	MAX_NODES int
	DM_PASSWORD string

	NL_MUTEX sync.Mutex
)

const (
	USER_AGENT = "whiteprotocol:whiteprotocol/v" + VERSION // Format: whiteprotocol:<client>/<version with v prefix>. Official client whiteprotocol is eponymous to the protocol "whiteprotocol"
	VERSION = "0.0.3"
)

type NODE struct {
	Conn chan string
	IP   string
}

type MESSAGE struct {
	FromIP 		string
	UserAgent 	string

	Task 		string
	TaskArgs 	[]string
}

type RESPONSE struct {
	FromIP 		string
	UserAgent 	string

	Response 	int
	ResponseData 	string
}