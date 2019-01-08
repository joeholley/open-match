package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/spf13/viper"
	"github.com/tidwall/gjson"
	"google.golang.org/grpc"

	backend "github.com/GoogleCloudPlatform/open-match/internal/pb"
)

func mustReadProfiles(cfg *viper.Viper) (starterProfile *backend.MatchObject, profiles []*backend.MatchObject) {
	if s := cfg.GetString("starter.profile"); s != "" {
		starterProfile = mustReadProfile(s)
	} else {
		panic("starter profile filename is empty or not set config")
	}

	if ss := cfg.GetStringSlice("profiles"); len(ss) > 0 {
		for _, s := range ss {
			p := mustReadProfile(s)
			profiles = append(profiles, p)
		}
	} else {
		panic("profiles' filenames are not specified in config")
	}
	return
}

func mustReadProfile(filename string) *backend.MatchObject {
	p, err := readProfile(filename)
	if err != nil {
		panic(fmt.Sprintf("error reading profile at \"%s\": %s", filename, err.Error()))
	}
	return p
}

func readProfile(filename string) (*backend.MatchObject, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file \"%s\": %s", filename, err.Error())
	}
	defer jsonFile.Close()

	// parse json data and remove extra whitespace before sending to the backend.
	jsonData, _ := ioutil.ReadAll(jsonFile) // this reads as a byte array
	buffer := new(bytes.Buffer)             // convert byte array to buffer to send to json.Compact()
	if err := json.Compact(buffer, jsonData); err != nil {
		dirLog.WithError(err).WithField("filename", filename).Warn("error compacting profile json")
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

func getBackendAPIClient() (*grpc.ClientConn, backend.BackendClient, error) {
	// Connect gRPC client
	addrs, err := net.LookupHost("om-backendapi")
	if err != nil {
		return nil, nil, errors.New("lookup failed: " + err.Error())
	}

	addr := fmt.Sprintf("%s:50505", addrs[0])

	beAPIConn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, errors.New("failed to connect: " + err.Error())
	}
	beAPI := backend.NewBackendClient(beAPIConn)
	dirLog.Debugf("API client connected to %s", addr)
	return beAPIConn, beAPI, nil
}
