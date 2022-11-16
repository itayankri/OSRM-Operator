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
const osrmPartitionedData = "partitioned"
const osrmCustomizedData = "customized"

const GatewaySuffix = "gateway"
const PersistentVolumeClaimSuffix = ""
const JobSuffix = "map-builder"
const CronJobSuffix = "speed-updates"
const DeploymentSuffix = ""
const HorizontalPodAutoscalerSuffix = ""
const PodDisruptionBudgetSuffix = ""
const ServiceSuffix = ""
const ConfigMapSuffix = ""

const nginxConfigurationTemplateName = "nginx.tmpl"
const gatewayImage = "nginx"

const lastTrafficUpdateTimeAnnotation = "osrmcluster.itayankri/lastTrafficUpdateTime"
