package resource

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

const gatewayPostfix = "gateway"
const nginxConfigurationFileName = "nginx.conf"
const gatewayImage = "nginx"
