module "github.com/booter/bmod" {
    name = "bmod"
    priority = GLOBAL_BASE_PRIORITY_APP + 2
    disabled = false
    config {
        Filename                     = "${env("HOME", ".")}/${GLOBAL_LOGDIR}/cmqd00.log"
        Append                       = GLOBAL_LOG_APPEND
        MaxBackups                   = anyname_MAX_BACKUPS
        RotateSchedule               = lower(anyname_ROTATE)
        DefaultLevel                 = flagOrError("--logging-default-level")
        DefaultPrefixWidth           = flag("--logging-default-prefix-width", GLOBAL_LOG_PREFIX_WIDTH)
        DefaultEnableSourceLocation  = flag("--logging-default-enable-source-location", true)
        Levels = [
            { Pattern="MCH_*", Level="DEBUG" },
            { Pattern="proc", Level="TRACE" },
            { Pattern="cemlib", Level="TRACE" },
        ]
    }
    // reference field "Bmod" infered by camel case of module name "bmod"
    inject amod Bmod {}

    // explicitly assigned field "OtherNameForBmod"
    inject amod OtherNameForBmod {}
}
