package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/tidwall/gjson"
	"google.golang.org/grpc"

	backend "github.com/GoogleCloudPlatform/open-match/internal/pb"
)

func readProfiles(filenames []string) ([]*backend.MatchObject, error) {
	var profiles []*backend.MatchObject
	for _, filename := range filenames {
		p, err := readProfile(filename)
		if err != nil {
			return profiles, fmt.Errorf("Error reading profile \"%s\": %s", filename, err.Error())
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func readProfile(filename string) (*backend.MatchObject, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, errors.New("Failed to open file specified at command line.  Did you forget to specify one?")
	}
	defer jsonFile.Close()

	// parse json data and remove extra whitespace before sending to the backend.
	jsonData, _ := ioutil.ReadAll(jsonFile) // this reads as a byte array
	buffer := new(bytes.Buffer)             // convert byte array to buffer to send to json.Compact()
	if err := json.Compact(buffer, jsonData); err != nil {
		log.Println(err)
	}

	jsonProfile := buffer.String()

	profileName := "test-dm-usc1f"
	if gjson.Get(jsonProfile, "name").Exists() {
		profileName = gjson.Get(jsonProfile, "name").String()
	}

	pbProfile := &backend.MatchObject{
		Id:         profileName,
		Properties: jsonProfile,
	}
	return pbProfile, nil
}

func getBackendAPIClient() (backend.BackendClient, error) {
	// Connect gRPC client
	addrs, err := net.LookupHost("om-backendapi")
	if err != nil {
		return nil, errors.New("lookup failed: " + err.Error())
	}

	beAPIConn, err := grpc.Dial(addrs[0]+":50505", grpc.WithInsecure())
	if err != nil {
		return nil, errors.New("failed to connect: " + err.Error())
	}
	beAPI := backend.NewBackendClient(beAPIConn)
	log.Println("API client connected to", addrs[0]+":50505")
	return beAPI, nil
}
