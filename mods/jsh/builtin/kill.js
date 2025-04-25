jsh = require("@jsh/process")
args = jsh.args()
if( args.length != 2) {
    jsh.print("Usage: kill <pid>","\n")
} else {
    pid = parseInt(args[1])
    ret = jsh.kill(pid)
    if(ret !== undefined && ret.value != undefined) {
        jsh.print(ret.value, "\n")
    }
}
