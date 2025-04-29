const {Table, ps} = require("@jsh/process")

tab = new Table()
tab.appendHeader("PID", "PPID", "User", "Name", "Uptime")
for( const p of ps() ) {
    ppid = "-"
    if ( p.ppid != 0xFFFFFFFF) {
        ppid = p.ppid
    }
    tab.appendRow(p.pid, ppid, p.user, p.name, p.uptime)
}
tab.render()
