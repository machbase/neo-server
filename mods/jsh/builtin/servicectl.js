p = require("@jsh/process")
var args = p.args()

function usage() {
    p.print("Usage: servicectl <command> [service]\n")
    p.print("Commands:\n")
    p.print("  status [service] - Show the status of a service or all services\n")
    p.print("  start <service> - Start a service\n")
    p.print("  stop <service> - Stop a service\n")
    p.print("  reread - Reread the configuration files\n")
    p.print("  update - Update the services\n")
}

function handleStatus(service) {
    tab = new p.Table()
    try {
        var list = p.serviceStatus(service)
        if (list == null || list.length == 0) {
            p.print("No services found\n")
        } else {
            tab.appendHeader("Service", "Status", "PID")
            for (s of list) {
                tab.appendRow(s.name, s.status, s.pid == 0 ? "" : s.pid)
            }
        }
    } catch (e) {
        p.print("Error:", e, "\n")
    } finally {
        tab.render()
    }
}

function handleStart(service) {
    try {
        p.serviceStart(service)
    } catch (e) {
        p.print("Error:", e, "\n")
    }
}

function handleStop(service) {
    try {
        p.serviceStop(service)
    } catch (e) {
        p.print("Error:", e, "\n")
    }
}

function handleReread() {
    tab = new p.Table()
    try {
        var result = p.serviceReread()
        if (result == null || result.length == 0) {
            p.print("No services found\n")
        } else {
            [
                ["Added", result.added],
                ["Changed", result.updated],
                ["Removed", result.removed],
                ["Unchanged", result.unchanged],
                ["Errored", result.errors],
            ].forEach((item) => {
                label = item[0]
                list = item[1]
                if(list.length == 0) {
                    return
                }
                tab.appendHeader(`${label} Service`, "Enable", "StartCmd", "StartArgs")
                for( s of list ) {
                    tab.appendRow(s.name, s.enable, s.start_cmd, s.start_args.join(", "))
                }
                tab.render()
                tab.resetRows()
                tab.resetHeaders()
            })
        }
    } catch (e) {
        p.print("Error:", e.message, "\n")
    } finally {
        tab.render()
    }
}

function handleUpdate() {
    try {
        var list = p.serviceUpdate()
        for (s of list) {
            p.print(s, "\n")
        }
    } catch (e) {
        p.print("Error:", e, "\n")
    }
}

if (args.length < 2 || ((args[1] == "start" || args[1] == "stop") && args.length < 3)) {
    usage()
} else {
    var command = args[1]
    var service = ""
    if (command == "start" || command == "stop") {
        service = args[2]
    }

    if (command == "status") {
        handleStatus(service)
    } else if (command == "reread") {
        handleReread()
    } else if (command == "update") {
        handleUpdate()
    } else if (command == "start") {
        handleStart(service)
    }
    else if (command == "stop") {
        handleStop(service)
    }
    else {
        p.print("Unknown command:", command, "\n")
        usage()
    }
}