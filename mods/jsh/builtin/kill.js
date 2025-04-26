jsh = require("@jsh/process")
args = jsh.args()
if( args.length != 2) {
    jsh.print("Usage: kill <pid>","\n")
} else {
    try {
        pid = parseInt(args[1])
        jsh.ps(pid)
        if( pid < 1024 ) {
            jsh.print("Cannot kill system process\n")
        } else {
            ret = jsh.kill(pid)
            if(ret !== undefined && ret.value != undefined) {
                jsh.print(ret.value, "\n")
            }    
        }
    } catch(e) {
        jsh.print(e.message, "\n")
    }
}
