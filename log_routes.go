package logging

import (
	"github.com/menucha-de/utils"
)

//LogRoutes routes
var LogRoutes = utils.Routes{

	utils.Route{
		Name:        "GetLogLevels",
		Method:      "GET",
		Pattern:     "/rest/log/levels",
		HandlerFunc: getLogLevels,
	},
	utils.Route{
		Name:        "GetLogTargets",
		Method:      "GET",
		Pattern:     "/rest/log/targets",
		HandlerFunc: getLogTargets,
	},
	utils.Route{
		Name:        "GetLevel",
		Method:      "GET",
		Pattern:     "/rest/log/{target}/level",
		HandlerFunc: getLogLevel,
	},
	utils.Route{
		Name:        "SetLogLevel",
		Method:      "PUT",
		Pattern:     "/rest/log/{target}/level",
		HandlerFunc: setLogLevel,
	},
	utils.Route{
		Name:        "GetLogSize",
		Method:      "GET",
		Pattern:     "/rest/log/{target}/{level}",
		HandlerFunc: getLogSize,
	},
	utils.Route{
		Name:        "GetLogEntries",
		Method:      "GET",
		Pattern:     "/rest/log/{target}/{level}/{limit}/{offset}",
		HandlerFunc: getLogEntries,
	},
	utils.Route{
		Name:        "DeleteLogEntries",
		Method:      "DELETE",
		Pattern:     "/rest/log/{target}",
		HandlerFunc: deleteLogEntries,
	},
	utils.Route{
		Name:        "GetLogFile",
		Method:      "GET",
		Pattern:     "/rest/log/{target}/{level}/export",
		HandlerFunc: getLogFile,
	},
}
