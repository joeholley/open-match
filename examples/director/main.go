package main

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/tidwall/gjson"

	"github.com/GoogleCloudPlatform/open-match/internal/logging"
	backend "github.com/GoogleCloudPlatform/open-match/internal/pb"

	director_config "github.com/GoogleCloudPlatform/open-match/examples/director/config"
)

// TODO vvv
const (
	maxSends          = 2
	maxMatchesPerSend = 2

	waitBetweenStartups = 2 * time.Second
	sleepBetweenSends   = 30 * time.Second
)

var (
	minPlayers int64
	maxWait    time.Duration

	namespace    string
	fleetName    string
	generateName string

	cfg = viper.New()

	// Logrus structured logging setup
	dirLogFields = log.Fields{
		"app":       "openmatch",
		"component": "director",
	}
	dirLog = log.WithFields(dirLogFields)

	err = errors.New("")
)

func init() {
	// Viper config management initialization
	cfg, err = director_config.Read()
	if err != nil {
		dirLog.WithError(err).Fatal("Unable to load config file")
	}

	dirLog.WithField("cfg", cfg.AllSettings()).Info("Working with configuration")

	minPlayers = cfg.GetInt64("starter.minPlayers")
	maxWait = time.Duration(cfg.GetInt64("starter.maxWaitSeconds") * int64(time.Second))

	if namespace = cfg.GetString("agones.namespace"); namespace == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.namespace\"")
	}
	if fleetName = cfg.GetString("agones.fleetName"); fleetName == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.fleetName\"")
	}
	if generateName = cfg.GetString("agones.generateName"); generateName == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.generateName\"")
	}

	// Configure open match logging defaults
	logging.ConfigureLogging(cfg)
}

func main() {
	emptyProfile, profiles := mustReadProfiles(cfg)

	// Sleep until there's enough players to submit real profiles
	_, err = requirePlayers(emptyProfile, minPlayers, maxWait)
	if err != nil {
		dirLog.WithError(err).Fatal("Error waiting for players")
	}

	dirLog.Info("Well, it looks like there is enough players to start matchmaking")

	// Start sending profiles concurrently
	var wg sync.WaitGroup
	for i, p := range profiles {
		dirLog.Debug("Go send profile \"%s\"!", p.Id)
		wg.Add(1)
		go func() {
			defer wg.Done()
			sendProfile(p, maxSends, maxMatchesPerSend)
		}()

		if i < len(profiles)-1 {
			time.Sleep(waitBetweenStartups)
		}
	}

	dirLog.Debug("Waiting for goroutines to exit...")

	wg.Wait()
	dirLog.Info("Exiting")
}

// Sends empty profile in order to get back some stats with total number of players in Redis
func requirePlayers(profile *backend.MatchObject, threshold int64, timeout time.Duration) (bool, error) {
	beAPI, err := getBackendAPIClient()
	if err != nil {
		return false, errors.New("Error creating Backend client: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stream, err := beAPI.ListMatches(ctx, profile)
	if err != nil {
		return false, errors.New("Error opening ListMatches stream: " + err.Error())
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

		dirLog.Debugf("match.Pools = %+v", match.Pools)

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

func sendProfile(profile *backend.MatchObject, maxSends int, maxMatchesPerSend int) {
	profLog := dirLog.WithField("profile", profile.Id)
	defer func() {
		profLog.Debug("Exiting")
	}()

	beAPI, err := getBackendAPIClient()
	if err != nil {
		profLog.WithError(err).Error("Error creating Backend client")
		return
	}

	for i := 0; i < maxSends; i++ {
		profLog.Debug("calling ListMatches...")
		sendLog := profLog.WithField("send", i)

		stream, err := beAPI.ListMatches(context.Background(), profile)
		if err != nil {
			sendLog.WithError(err).Error("Error opening ListMatches stream")
			return
		}

		j := -1
		for {
			j++
			recvLog := sendLog.WithField("recv", j)
			if j >= maxMatchesPerSend {
				recvLog.Debug("Reached max num of match receive attempts, closing stream...")
				stream.CloseSend()
				break
			}

			recvLog.Debug("Calling stream.Recv()...")
			match, err := stream.Recv()
			if err == io.EOF {
				sendLog.Debug("ListMatches stream is closed")
				break
			}

			if err != nil {
				recvLog.WithError(err).Error("Error receiving match")
				stream.CloseSend()
				break
			}

			if match.Error != "" {
				recvLog.WithField(log.ErrorKey, match.Error).Error("Received a match with non-empty error")
				stream.CloseSend()
				break
			}

			matchLog := recvLog.WithField("match", match.Id)
			matchLog.Debug("Received")

			if !gjson.Valid(string(match.Properties)) {
				matchLog.Error("Invalid properties json")
				stream.CloseSend()
				break
			}

			var connstring string
			connstring, err = allocate(match)
			if err != nil {
				matchLog.WithError(err).Error("error allocating match")
				stream.CloseSend()
				break
			}

			// Get players from the json properties.roster field
			players := make([]string, 0)
			result := gjson.Get(match.Properties, "properties.rosters")
			result.ForEach(func(_, teamRoster gjson.Result) bool {
				teamPlayers := teamRoster.Get("players")
				teamPlayers.ForEach(func(_, teamPlayer gjson.Result) bool {
					player := teamPlayer.Get("id")
					players = append(players, player.String())
					return true
				})
				return true // keep iterating
			})

			matchLog.WithFields(log.Fields{
				"players":    players,
				"connstring": connstring,
			}).Info("Assigning players to DGS...")

			assign := &backend.Assignments{Rosters: match.Rosters, Assignment: connstring}
			_, err = beAPI.CreateAssignments(context.Background(), assign)
			if err != nil {
				matchLog.WithError(err).Error("Error creating assignments")
				break
			}
		}

		if i < maxSends-1 {
			sendLog.Println("Sleeping...")
			time.Sleep(sleepBetweenSends)
		}
	}
}

// Tries to allocate DGS and distributes matchmaking results to it
func allocate(match *backend.MatchObject) (string, error) {
	fa, err := allocateDGS(namespace, fleetName, generateName)
	if err != nil {
		return "", err
	}

	// TODO distribute the results to DGS

	connstring := getConnectionString(fa)
	return connstring, nil
}
