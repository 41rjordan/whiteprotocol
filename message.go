package main

import (
	"encoding/json"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"io/ioutil"
	"errors"
)

func fileExists(filename string) bool {
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func createMessageKeypair() error {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	encodedPublicKey := base64.StdEncoding.EncodeToString(publicKey)
	encodedPrivateKey := base64.StdEncoding.EncodeToString(privateKey)
	serializedKeypair, _ := json.Marshal(MESSAGEKEYPAIR{encodedPublicKey, encodedPrivateKey})

	file, err := os.Create("keypair")
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = file.WriteString(string(serializedKeypair))
	if err != nil {
		return err
	}

	return nil
}

func getMessageKeypair() (MESSAGEKEYPAIR, error) {
	var keypair MESSAGEKEYPAIR

	data, err := ioutil.ReadFile("keypair")
	if err != nil {
		return keypair, err
	}

	err = json.Unmarshal(data, &keypair)
	if err != nil {
		return keypair, err
	}

	return keypair, nil
}

func signMessage(encodedPrivateKey string, message []byte) []byte {
	privateKey, err := base64.StdEncoding.DecodeString(encodedPrivateKey)
	if err != nil {
		panic(err)
	}
	signature := ed25519.Sign(privateKey, message)
	return signature
}

func verifyMessageSignature(message []byte, publicKey []byte) bool {
	return ed25519.Verify(publicKey, message[64:], message[:64])
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
	keypair, err := getMessageKeypair()
	if err != nil {
		panic(err)
	}

	result, _ := json.Marshal(response)
	result = append(signMessage(keypair.PrivateKey, result), result...)

	return append(result, []byte{10}...)
}

func deserializeResponse(serializedResponse []byte) (RESPONSE, error) {
	var response RESPONSE
	err := json.Unmarshal(serializedResponse, &response)
	
	return response, err
}