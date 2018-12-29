package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/tidwall/gjson"

	backend "github.com/GoogleCloudPlatform/open-match/internal/pb"
)

const (
	minPlayers = 20
	maxWait    = 1 * time.Hour

	maxSends          = 2
	maxMatchesPerSend = 2

	waitBetweenStartups = 2 * time.Second
	sleepBetweenSends   = 30 * time.Second

	namespace    = "default"
	fleetname    = "udp-server"
	generatename = "udp-server-"
)

func main() {
	profiles, err := readProfiles([]string{
		"profiles/testprofile.json",
		"profiles/something.json",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Sleep until there's enough players to submit real profiles
	_, err = requirePlayers(minPlayers, maxWait)
	if err != nil {
		log.Fatalf("Error waiting for players: %v", err.Error())
	}

	log.Println("Well, it looks like there is enough players to start matchmaking")

	// Start sending profiles concurrently
	var wg sync.WaitGroup
	for i, p := range profiles {
		log.Printf("Go send profile \"%s\"!", p.Id)
		wg.Add(1)
		go func() {
			defer wg.Done()
			doSendProfile(p, maxSends, maxMatchesPerSend)
		}()

		if i < len(profiles)-1 {
			time.Sleep(waitBetweenStartups)
		}
	}

	log.Println("Waiting for goroutines to exit...")

	wg.Wait()
	log.Println("Exiting")
}

// Sends empty profile in order to get back some stats with total number of players in Redis
func requirePlayers(threshold int64, timeout time.Duration) (bool, error) {
	profile, err := readProfile("profiles/__empty__.json")
	if err != nil {
		return false, errors.New("error reading empty profile: " + err.Error())
	}

	beAPI, err := getBackendAPIClient()
	if err != nil {
		log.Println("error creating Backend client:", err)
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stream, err := beAPI.ListMatches(ctx, profile)
	if err != nil {
		log.Println("error opening ListMatches stream:", err.Error())
		return false, err
	}

	var enough bool
	for {
		match, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, errors.New("error reading stream for ListMatches(_): " + err.Error())
		}

		log.Printf("match.Pools = %+v", match.Pools)

		var defaultPoolPlayersCount int64
		for _, p := range match.Pools {
			if p.Name == "defaultPool" {
				if p.Stats != nil {
					defaultPoolPlayersCount = p.Stats.Count
				}
			}
		}
		if defaultPoolPlayersCount > threshold {
			enough = true
			err := stream.CloseSend()
			if err != nil {
				return enough, errors.New("error closing stream for ListMatches(_): " + err.Error())
			}
			break
		}
	}
	return enough, nil
}

func doSendProfile(profile *backend.MatchObject, maxSends int, maxMatchesPerSend int) {
	p := fmt.Sprintf("Profile \"%s\":", profile.Id)
	defer func() {
		log.Println(p, "exiting")
	}()

	beAPI, err := getBackendAPIClient()
	if err != nil {
		log.Println(p, "error creating Backend client:", err)
		return
	}

	for i := 0; i < maxSends; i++ {
		p_ := fmt.Sprintf("%s #%d:", p, i)

		log.Println(p, "calling ListMatches...")
		stream, err := beAPI.ListMatches(context.Background(), profile)
		if err != nil {
			log.Println(p_, "error opening ListMatches stream:", err.Error())
			return
		}

		j := -1
		for {
			j++
			if j >= maxMatchesPerSend {
				log.Println(p, "reached max num of match receive attempts, closing stream...")
				stream.CloseSend()
				break
			}

			log.Println(p, "calling stream.Recv()...")
			match, err := stream.Recv()
			if err == io.EOF {
				log.Println(p_, "ListMatches stream is closed")
				break
			}

			if err != nil {
				log.Println(p_, "error receiving match:", err)
				stream.CloseSend()
				break
			}

			if match.Error != "" {
				log.Println(p_, "received match with non-empty error:", match.Error)
				stream.CloseSend()
				break
			}

			p__ := fmt.Sprintf("%s match \"%s\":", p_, match.Id)
			log.Println(p__, "received")

			if !gjson.Valid(string(match.Properties)) {
				log.Println(p__, "invalid properties json")
				stream.CloseSend()
				break
			}

			var connstring string
			connstring, err = allocate(match, p__)
			if err != nil {
				log.Println(p__, "error allocating match:", err)
				stream.CloseSend()
				break
			}

			// Get players from the json properties.roster field
			players := make([]string, 0)
			result := gjson.Get(match.Properties, "properties.roster")
			result.ForEach(func(teamName, teamRoster gjson.Result) bool {
				teamRoster.ForEach(func(_, player gjson.Result) bool {
					players = append(players, player.String())
					return true // keep iterating
				})
				return true // keep iterating
			})

			log.Printf(p__+"assigning %d players to %s", len(players), connstring)
			assign := &backend.Assignments{Rosters: match.Rosters, Assignment: connstring}
			_, err = beAPI.CreateAssignments(context.Background(), assign)
			if err != nil {
				log.Println(p__, "error creating assignments:", err)
				break
			}
		}

		if i < maxSends-1 {
			log.Println(p_, "Sleeping...")
			time.Sleep(sleepBetweenSends)
		}
	}
}

// Tries to allocate DGS and distributes matchmaking results to it
func allocate(match *backend.MatchObject, p__ string) (string, error) {
	fa, err := allocateDGS(namespace, fleetname, generatename)
	if err != nil {
		return "", err
	}

	// TODO distribute the results to DGS

	connstring := getConnectionString(fa)
	log.Println(p__, "DGS allocated:", fa.Status.GameServer.Name)
	return connstring, nil
}
