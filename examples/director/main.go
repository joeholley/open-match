package main

import (
	"context"
	"errors"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/GoogleCloudPlatform/open-match/internal/logging"
	backend "github.com/GoogleCloudPlatform/open-match/internal/pb"

	director_config "github.com/GoogleCloudPlatform/open-match/examples/director/config"
)

// Allocator is the interface that allocates a game server for a match object
type Allocator interface {
	Allocate(match *backend.MatchObject) (string, error)
	UnAllocate(string) error
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
		"starter.minPlayers": minPlayers,
		"starter.maxWait":    maxWait,

		"debug.maxSends":          maxSends,
		"debug.maxMatchesPerSend": maxMatchesPerSend,
		"debug.sleepBetweenSends": sleepBetweenSends,

		"agones.namespace":    namespace,
		"agones.fleetName":    fleetName,
		"agones.generateName": generateName,
	}).Debug("Parameters read from configuration")
}

func main() {
	starterProfile, profiles := mustReadProfiles(cfg)

	ctx := context.Background()

	// Sleep until there's enough players to submit real profiles
	waitForPlayers(ctx, starterProfile)

	dirLog.Info("Well, it looks like there is enough players to start matchmaking")

	// Start sending profiles concurrently
	var wg sync.WaitGroup
	for _, p := range profiles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startSendProfile(ctx, p, dirLog)
		}()
	}

	wg.Wait()
	dirLog.Info("Exiting")
}

// Sends provided "starter" profile and receives match objects
// until number of players in "defaultPool" passes threshold value (or time is over)
func waitForPlayers(ctx context.Context, profile *backend.MatchObject) {
	ctx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()

	err := listMatches(ctx, profile, func(match *backend.MatchObject) (bool, error) {
		dirLog.Debugf("match.Pools = %+v", match.Pools)
		return countPlayers(match) < minPlayers, nil
	})
	if err != nil {
		dirLog.WithError(err).Fatal("Error waiting for players")
	}
}
