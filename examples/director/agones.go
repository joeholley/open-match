package main

import (
	"errors"
	"fmt"

	"agones.dev/agones/pkg/apis/stable/v1alpha1"
	"agones.dev/agones/pkg/client/clientset/versioned"
	"agones.dev/agones/pkg/util/runtime" // for the logger
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

var (
	logger       = runtime.NewLoggerWithSource("main")
	agonesClient = getAgonesClient()
)

// Set up our client which we will use to call the API
func getAgonesClient() *versioned.Clientset {
	// Create the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.WithError(err).Fatal("Could not create in cluster config")
	}

	// Access to the Agones resources through the Agones Clientset
	agonesClient, err := versioned.NewForConfig(config)
	if err != nil {
		logger.WithError(err).Fatal("Could not create the agones api clientset")
	} else {
		logger.Info("Created the agones api clientset")
	}
	return agonesClient
}

// Return the number of ready game servers available to this fleet for allocation
func checkReadyReplicas(namespace, fleetname string) int32 {
	// Get a FleetInterface for this namespace
	fleetInterface := agonesClient.StableV1alpha1().Fleets(namespace)
	// Get our fleet
	fleet, err := fleetInterface.Get(fleetname, v1.GetOptions{})
	if err != nil {
		logger.WithError(err).Info("Get fleet failed")
	}

	return fleet.Status.ReadyReplicas
}

// Move a replica from ready to allocated and return the GameServerStatus
func allocateDGS(namespace, fleetname, generatename string) (*v1alpha1.FleetAllocation, error) {
	// var result v1alpha1.GameServerStatus

	// Log the values used in the fleet allocation
	// logger.WithField("namespace", namespace).Info("namespace for fa")
	// logger.WithField("generatename", generatename).Info("generatename for fa")
	// logger.WithField("fleetname", fleetname).Info("fleetname for fa")

	// Find out how many ready replicas the fleet has - we need at least one
	readyReplicas := checkReadyReplicas(namespace, fleetname)
	logger.WithField("readyReplicas", readyReplicas).Info("numer of ready replicas")

	// Log and return an error if there are no ready replicas
	if readyReplicas < 1 {
		// logger.WithField("fleetname", fleetname).Info("Insufficient ready replicas, cannot create fleet allocation")
		return nil, errors.New("Insufficient ready replicas, cannot create fleet allocation")
	}

	// Get a FleetAllocationInterface for this namespace
	fleetAllocationInterface := agonesClient.StableV1alpha1().FleetAllocations(namespace)

	// Define the fleet allocation using the constants set earlier
	fa := &v1alpha1.FleetAllocation{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: generatename, Namespace: namespace,
		},
		Spec: v1alpha1.FleetAllocationSpec{FleetName: fleetname},
	}

	// Create a new fleet allocation
	newFleetAllocation, err := fleetAllocationInterface.Create(fa)
	if err != nil {
		// Log and return the error if the call to Create fails
		logger.WithError(err).Info("Failed to create fleet allocation")
		return nil, errors.New("Failed to create fleet allocation: " + err.Error())
	}

	// Log the GameServer.Staus of the new allocation, then return those values
	// logger.Info("New GameServer allocated: ", newFleetAllocation.Status.GameServer.Name)
	// result = newFleetAllocation.Status.GameServer.Status
	return newFleetAllocation, nil
}

func getConnectionString(fa *v1alpha1.FleetAllocation) string {
	dgs := fa.Status.GameServer.Status
	return fmt.Sprintf("%s:%d", dgs.Address, dgs.Ports[0].Port)
}
