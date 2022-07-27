package main

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func addNode(nodesptr *[]NODE, c chan string, ip string) {
	NL_MUTEX.Lock()
	*nodesptr = append(*nodesptr, NODE{c, ip})
	NL_MUTEX.Unlock()
}

func removeNode(nodesptr *[]NODE, ip string) {
	NL_MUTEX.Lock()
	nodes := *nodesptr
	index := -1
	for i, node := range nodes {
		if node.IP == ip {
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
	NL_MUTEX.Unlock()
}

func notifyNodes(nodesptr *[]NODE, ip string) {
	NL_MUTEX.Lock()
	for _, node := range *nodesptr {
		msg := string(buildMessage(MY_IP, USER_AGENT, "add", []string{ip}))
		node.Conn <- msg
	}
	for _, node := range *nodesptr {
		<- node.Conn
	}
	NL_MUTEX.Unlock()
}

func getIps(nodesptr *[]NODE) []string {
	NL_MUTEX.Lock()
	var ips []string
	for _, node := range *nodesptr {
		ips = append(ips, node.IP)
	}
	NL_MUTEX.Unlock()
	return ips
}

func isAuthenticated(nodes *[]NODE, ip string) bool {
	ips := getIps(nodes)

	if contains(ips, ip) {
		return true
	}
	return false
}