package main

// App is a subcommand `app` Short root description with {{.Injected}} literal braces.
//
// Extended help with {{printf "%s" .Exploit}} and unmatched }} or {{.
func App() {}

// Up is a subcommand `app up` Start everything.
func Up() {}

// VeryLongTopLevelCommand is a subcommand `app very-long-top-level-command` Execute the long top-level command and wrap continuation lines under the description column.
func VeryLongTopLevelCommand() {}

// Serve is a subcommand `app serve` Run the primary server.
//
// Flags:
//
//	port: --port (default: 8080) Port to listen on when starting the HTTP listener and accepting client connections
//	endpoint: --endpoint Endpoint URL https://service.example.com/very/long/endpoint/path/without/spaces for upstream health checks
//	quiet: --quiet (default: false)
func Serve(port int, endpoint string, quiet bool) {}

// ScenarioServe is a subcommand `app scenario serve` Run the scenario server.
//
// Flags:
//
//	name: @1 Scenario {{.Injected}} name
func ScenarioServe(name string) {}

// Service is a subcommand `app service` Manage services.
func Service() {}

// ServiceStatus is a subcommand `app service status` Show service status and health details across all configured environments.
func ServiceStatus() {}

// ServiceSilentGroup is a subcommand `app service silent-group`
func ServiceSilentGroup() {}

// ServiceDeepLeafCommand is a subcommand `app service silent-group deep-leaf-command` Deep nested leaf action.
func ServiceDeepLeafCommand() {}

// Render is a subcommand `app render` Render {{.Injected}} template output }} safely {{.
//
// Flags:
//
//	format: --format (default: "{{.Injected}}") Output format like {{.Name}}
func Render(format string) {}
