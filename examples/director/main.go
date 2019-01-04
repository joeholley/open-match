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

// Allocator is the interface that allocates a game server for a match object
type Allocator interface {
	Allocate(match *backend.MatchObject) (string, error)
}

var (
	// Starter config
	minPlayers int64
	maxWait    time.Duration

	// Profiles debugging config:
	maxSends            int
	maxMatchesPerSend   int
	sleepBetweenSends   = 30 * time.Second
	waitBetweenStartups = 2 * time.Second

	allocator Allocator

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

	// Configure open match logging defaults
	logging.ConfigureLogging(cfg)

	dirLog.WithField("cfg", cfg.AllSettings()).Info("Configuration provided")

	// Starter config
	minPlayers = cfg.GetInt64("starter.minPlayers")
	maxWait = time.Duration(cfg.GetInt64("starter.maxWaitSeconds") * int64(time.Second))

	// Profiles debugging
	maxSends = cfg.GetInt("debug.maxSends")
	maxMatchesPerSend = cfg.GetInt("debug.maxMatchesPerSend")
	sleepBetweenSends = time.Duration(cfg.GetInt64("debug.sleepBetweenSendsSeconds") * int64(time.Second))
	waitBetweenStartups = time.Duration(cfg.GetInt64("debug.waitBetweenStartupsSeconds") * int64(time.Second))

	// Agones
	var namespace, fleetName, generateName string
	if namespace = cfg.GetString("agones.namespace"); namespace == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.namespace\"")
	}
	if fleetName = cfg.GetString("agones.fleetName"); fleetName == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.fleetName\"")
	}
	if generateName = cfg.GetString("agones.generateName"); generateName == "" {
		dirLog.Fatalf("Incomplete Agones configuration: missing \"agones.generateName\"")
	}
	allocator, err = NewAgonesAllocator(namespace, fleetName, generateName, dirLog)
	if err != nil {
		dirLog.WithError(err).Fatal("Could not create Agones allocator")
	}

	dirLog.WithFields(log.Fields{
		"minPlayers": minPlayers,
		"maxWait":    maxWait,

		"maxSends":            maxSends,
		"maxMatchesPerSend":   maxMatchesPerSend,
		"sleepBetweenSends":   sleepBetweenSends,
		"waitBetweenStartups": waitBetweenStartups,

		"namespace":    namespace,
		"fleetName":    fleetName,
		"generateName": generateName,
	}).Debug("Parameters read from configuration")
}

func main() {
	starterProfile, profiles := mustReadProfiles(cfg)

	// Sleep until there's enough players to submit real profiles
	err = waitForPlayers(starterProfile, minPlayers, maxWait)
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

// Sends provided "starter" profile and receives match objects
// until number of players in "defaultPool" passes threshold value (or time is over)
func waitForPlayers(profile *backend.MatchObject, minPlayers int64, maxWait time.Duration) error {
	beAPI, err := getBackendAPIClient()
	if err != nil {
		return errors.New("error creating backend client: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), maxWait)
	defer cancel()

	stream, err := beAPI.ListMatches(ctx, profile)
	if err != nil {
		return errors.New("error opening matches stream: " + err.Error())
	}

	for {
		match, err := stream.Recv()
		if err != nil {
			return errors.New("error reading matches stream: " + err.Error())
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
		if defaultPoolPlayersCount > minPlayers {
			_ = stream.CloseSend()
			return nil
		}
	}
}

func sendProfile(profile *backend.MatchObject, maxSends int, maxMatchesPerSend int) {
	profLog := dirLog.WithField("profile", profile.Id)
	defer func() {
		profLog.Debug("Exiting")
	}()

	beAPI, err := getBackendAPIClient()
	if err != nil {
		profLog.WithError(err).Error("error creating Backend client")
		return
	}

	for i := 0; i < maxSends || maxSends <= 0; i++ {
		profLog.Debug("Sending profile...")
		sendLog := profLog.WithField("send", i)

		stream, err := beAPI.ListMatches(context.Background(), profile)
		if err != nil {
			sendLog.WithError(err).Error("error opening matches stream")
			return
		}

		for j := 0; ; j++ {
			recvLog := sendLog.WithField("recv", j)
			if j >= maxMatchesPerSend && maxMatchesPerSend > 0 {
				recvLog.Debug("Reached max num of match receive attempts, closing stream...")
				stream.CloseSend()
				break
			}

			match, err := stream.Recv()
			if err == io.EOF {
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

			if !gjson.Valid(string(match.Properties)) {
				matchLog.Error("Invalid properties json")
				stream.CloseSend()
				break
			}

			var connstring string
			connstring, err = allocator.Allocate(match)
			if err != nil {
				matchLog.WithError(err).Error("error allocating match")
				stream.CloseSend()
				break
			}

			players := getPlayers(match)
			fields := log.Fields{"players": players, "connstring": connstring}

			assign := &backend.Assignments{Rosters: match.Rosters, Assignment: connstring}
			_, err = beAPI.CreateAssignments(context.Background(), assign)
			if err != nil {
				matchLog.WithFields(fields).WithError(err).Error("Error creating assignments")
				break
			} else {
				matchLog.WithFields(fields).Infof("Assigned %d players to %s", len(players), connstring)
			}
		}

		if i < maxSends-1 || maxSends <= 0 {
			sendLog.Debug("Sleeping...")
			time.Sleep(sleepBetweenSends)
		}
	}
}

// Gets players from the json properties.roster field
func getPlayers(match *backend.MatchObject) []string {
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
	return players
}
