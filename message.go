package main

import (
	"strings"
	"encoding/json"
)

func stringTaskToMessage(fromIP string, userAgent string, stringTask string) MESSAGE {
	stringArgs := strings.Split(stringTask, " ")
	message := MESSAGE{
		FromIP: fromIP,
		UserAgent: userAgent,
		Task: stringArgs[0],
		TaskArgs: stringArgs[1:],
	}
	return message
}

func buildMessage(fromIP string, userAgent string, task string, taskArgs []string) []byte {
	message := MESSAGE{
		FromIP: fromIP,
		UserAgent: userAgent,
		Task: task,
		TaskArgs: taskArgs,
	}
	return serializeMessage(&message)
}

func serializeMessage(message *MESSAGE) []byte {
	result, _ := json.Marshal(message)
	return append(result, []byte{10}...)
}

func deserializeMessage(serializedMessage string) (MESSAGE, error) {
	var message MESSAGE
	err := json.Unmarshal([]byte(serializedMessage), &message)
	
	return message, err
}

func buildResponse(fromIP string, userAgent string, responseID string, responseData string) []byte {
	response := RESPONSE{
		FromIP: fromIP,
		UserAgent: userAgent,
		Response: errorCodes[responseID],
		ResponseData: responseData,
	}
	return serializeResponse(&response)
}

func serializeResponse(response *RESPONSE) []byte {
	result, _ := json.Marshal(response)
	return append(result, []byte{10}...)
}

func deserializeResponse(serializedResponse string) (RESPONSE, error) {
	var response RESPONSE
	err := json.Unmarshal([]byte(serializedResponse), &response)
	
	return response, err
}