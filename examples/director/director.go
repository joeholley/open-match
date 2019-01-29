package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/cenkalti/backoff"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"

	backend "github.com/GoogleCloudPlatform/open-match/internal/pb"
)

func startSendProfile(ctx context.Context, profile *backend.MatchObject, l *log.Entry) {
	for i := 0; i < maxSends || maxSends <= 0; i++ {
		sendLog := l.WithFields(log.Fields{
			"profile": profile.Id,
			"#send":   i,
		})

		sendLog.Debugf("Sending profile \"%s\" (attempt #%d/%d)...", profile.Id, i+1, maxSends)
		sendProfile(ctx, profile, sendLog)

		if i < maxSends-1 || maxSends <= 0 {
			sendLog.Debugf("Sleeping \"%s\"...", profile.Id)
			time.Sleep(sleepBetweenSends)
		}
	}
}

func sendProfile(ctx context.Context, profile *backend.MatchObject, l *log.Entry) {
	var j int

	err := listMatches(ctx, profile, func(match *backend.MatchObject) (bool, error) {
		matchLog := l.WithFields(log.Fields{
			"#recv": j,
			"match": match.Id,
		})

		// Further actions may depend on the type of error received
		if match.Error != "" {
			matchLog.WithField(log.ErrorKey, match.Error).Error("Received a match with non-empty error, skip this match")
			return true, nil
		}
		if !gjson.Valid(string(match.Properties)) {
			matchLog.Error("Invalid properties json, skip this match")
			return true, nil
		}

		go allocateForMatch(ctx, match, matchLog)

		if j++; j >= maxMatchesPerSend && maxMatchesPerSend > 0 {
			matchLog.Debug("Reached max num of match receive attempts, closing stream")
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		l.WithError(err).Error(err)
	}
}

func allocateForMatch(ctx context.Context, match *backend.MatchObject, l *log.Entry) {
	// Try to allocate a DGS; retry with exponential backoff and jitter
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 2 * time.Second
	b.MaxElapsedTime = 2 * time.Minute
	bt := backoff.NewTicker(backoff.WithContext(b, ctx))

	var connstring string
	for range bt.C {
		connstring, err = allocator.Allocate(match)
		if err != nil {
			l.Error("Allocation attempt failed")
			continue
		}
		bt.Stop()
		break
	}
	if err != nil {
		l.WithError(err).Error("Could not allocate, match will be deleted")
		beAPIConn, beAPI, err := getBackendAPIClient()
		if err != nil {
			l.WithError(err).Error("error creating Backend client")
			return
		}
		defer beAPIConn.Close()

		_, err = beAPI.DeleteMatch(ctx, match)
		if err != nil {
			l.WithError(err).Error("Error deleting match after failed allocation attempt")
		}
		return
	}

	players := getPlayers(match)
	fields := log.Fields{"players": players, "connstring": connstring}

	beAPIConn, beAPI, err := getBackendAPIClient()
	if err != nil {
		l.WithError(err).Error("error creating Backend client")
		return
	}
	defer beAPIConn.Close()

	_, err = beAPI.CreateAssignments(ctx, &backend.Assignments{Rosters: match.Rosters, Assignment: connstring})
	if err != nil {
		l.WithFields(fields).WithError(err).Error("Error creating assignments, game server will be unallocated")
		err = allocator.UnAllocate(connstring)
		if err != nil {
			l.WithField("connstring", connstring).WithError(err).Error("Error unallocating game server after failed attempt to create assignments")
		}
		return
	}
	l.WithFields(fields).Infof("Assigned %d players to %s", len(players), connstring)
}

// MatchFunc is a function that is applied to each item of ListMatches() stream.
// Iteration is stopped and stream is closed if function return false or error.
type MatchFunc func(*backend.MatchObject) (bool, error)

func listMatches(ctx context.Context, profile *backend.MatchObject, fn MatchFunc) error {
	beAPIConn, beAPI, err := getBackendAPIClient()
	if err != nil {
		return errors.New("error creating Backend API client: " + err.Error())
	}
	defer beAPIConn.Close()

	stream, err := beAPI.ListMatches(ctx, profile)
	if err != nil {
		return errors.New("error opening matches stream: " + err.Error())
	}

	for {
		match, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			stream.CloseSend()
			return errors.New("error receiving match: " + err.Error())
		}

		var ok bool
		if ok, err = fn(match); err != nil {
			stream.CloseSend()
			return errors.New("error processing match: " + err.Error())
		}
		if !ok {
			stream.CloseSend()
			return nil
		}
	}
}
