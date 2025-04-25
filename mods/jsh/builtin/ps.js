jsh = require("@jsh/process")

jsh.print("  pid     ppid   name                     uptime\n")
jsh.print("------- ------- ------------------------ -----------\n")
for( const p of jsh.ps() ) {
    jsh.print(
        (""+p.pid).padStart(7, " "), 
        p.ppid == 0xFFFF ? "     - " : (""+p.ppid).padStart(7, " "),
        p.name.padEnd(24, " "),
        p.uptime,
        "\n")
}
jsh.print("\n")
