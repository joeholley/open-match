package main

import (
	"errors"
	"fmt"

	"agones.dev/agones/pkg/apis/stable/v1alpha1"
	"agones.dev/agones/pkg/client/clientset/versioned"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	backend "github.com/GoogleCloudPlatform/open-match/internal/pb"
)

// AgonesAllocator allocates game servers in Agones fleet
type AgonesAllocator struct {
	agonesClient *versioned.Clientset

	namespace    string
	fleetName    string
	generateName string

	logger *logrus.Entry
}

// NewAgonesAllocator creates new AgonesAllocator with in cluster k8s config
func NewAgonesAllocator(namespace, fleetName, generateName string, logger *logrus.Entry) (*AgonesAllocator, error) {
	agonesClient, err := getAgonesClient()
	if err != nil {
		return nil, errors.New("Could not create Agones allocator: " + err.Error())
	}

	a := &AgonesAllocator{
		agonesClient: agonesClient,

		namespace:    namespace,
		fleetName:    fleetName,
		generateName: generateName,

		logger: logger.WithFields(logrus.Fields{
			"source":    "agones",
			"namespace": namespace,
			"fleetname": fleetName,
		}),
	}
	return a, nil
}

// Set up our client which we will use to call the API
func getAgonesClient() (*versioned.Clientset, error) {
	// Create the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.New("Could not create in cluster config: " + err.Error())
	}

	// Access to the Agones resources through the Agones Clientset
	agonesClient, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, errors.New("Could not create the agones api clientset: " + err.Error())
	}
	return agonesClient, nil
}

// Allocate allocates a game server in a fleet, distributes match object details to it,
// and returns a connection string or error
func (a *AgonesAllocator) Allocate(match *backend.MatchObject) (string, error) {
	fa, err := a.allocateFleet()
	if err != nil {
		return "", err
	}

	// TODO distribute the results to DGS

	dgs := fa.Status.GameServer.Status
	connstring := fmt.Sprintf("%s:%d", dgs.Address, dgs.Ports[0].Port)
	return connstring, nil
}

// Move a replica from ready to allocated and return the GameServerStatus
func (a *AgonesAllocator) allocateFleet() (*v1alpha1.FleetAllocation, error) {
	// Find out how many ready replicas the fleet has - we need at least one
	readyReplicas := a.checkReadyReplicas()
	a.logger.WithField("readyReplicas", readyReplicas).Info("numer of ready replicas")

	// Log and return an error if there are no ready replicas
	if readyReplicas < 1 {
		return nil, errors.New("Insufficient ready replicas, cannot create fleet allocation")
	}

	// Get a FleetAllocationInterface for this namespace
	fai := a.agonesClient.StableV1alpha1().FleetAllocations(a.namespace)

	// Define the fleet allocation using the constants set earlier
	fa := &v1alpha1.FleetAllocation{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: a.generateName, Namespace: a.namespace,
		},
		Spec: v1alpha1.FleetAllocationSpec{FleetName: a.fleetName},
	}

	// Create a new fleet allocation
	newFleetAllocation, err := fai.Create(fa)
	if err != nil {
		// Log and return the error if the call to Create fails
		return nil, errors.New("Failed to create fleet allocation: " + err.Error())
	}

	a.logger.WithField("gsName", newFleetAllocation.Status.GameServer.Name).Info("DGS allocated")

	// Log the GameServer.Staus of the new allocation, then return those values
	// logger.Info("New GameServer allocated: ", newFleetAllocation.Status.GameServer.Name)
	// result = newFleetAllocation.Status.GameServer.Status
	return newFleetAllocation, nil
}

// Return the number of ready game servers available to this fleet for allocation
func (a *AgonesAllocator) checkReadyReplicas() int32 {
	// Get a FleetInterface for this namespace
	fleetInterface := a.agonesClient.StableV1alpha1().Fleets(a.namespace)
	// Get our fleet
	fleet, err := fleetInterface.Get(a.fleetName, v1.GetOptions{})
	if err != nil {
		a.logger.WithError(err).Error("Get fleet failed")
		return -1
	}
	return fleet.Status.ReadyReplicas
}
