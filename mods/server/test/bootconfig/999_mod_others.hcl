
module "github.com/booter/other" {
    disabled = true
    priority = GLOBAL_BASE_PRIORITY_APP+10
    config {
        Config = "../../test/other/config.ini"
    }
}