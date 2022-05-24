package status

const (
	AllProfilesReady         OSRMClusterConditionType = "AllProfilesReady"
	ClusterAvailable         OSRMClusterConditionType = "ClusterAvailable"
	ReconciliationSuccess    OSRMClusterConditionType = "ReconciliationSuccess"
	ReconciliationInProgress OSRMClusterConditionType = "ReconciliationInProgress"
)

type OSRMClusterConditionType string
