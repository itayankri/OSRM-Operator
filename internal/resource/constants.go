package resource

const (
	DrivingProfile OSRMProfile = "driving"
	CyclingProfile OSRMProfile = "cycling"
	FootProfile    OSRMProfile = "foot"
)

const (
	NearestService OSRMService = "nearest"
	RouteService   OSRMService = "route"
	TableService   OSRMService = "table"
	MatchService   OSRMService = "match"
	TripService    OSRMService = "trip"
	TileService    OSRMService = "tile"
)

const osrmContainerName = "osrm-backend"
const osrmDataVolumeName = "osrm-data"
const osrmDataPath = "/data"
