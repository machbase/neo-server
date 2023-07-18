

## Config file syntax

### `define <PREFIX>`

`define` block defines variables to be applied in the other parts of config file.

The syntax is `define <PREFIX>`, that comes with `<PREFIX>` which is identified for the block. 
Any string of alpabet can be used for a prefix.

When any value from other block needs to refer an item of defined block,
it will be concatenates with the prefix and `_` like `PREFIX_{item name}`.

As `VARS` block is defined as below example,
actual value of `VARS_IP_ADDR` is "127.0.0.1".

```hcl
define VARS {
    IP_ADDR     = "127.0.0.1"
    DEBUG_MODE  = true
    MAX_BACKUPS = 10
    LOG_DIR     = "./logs"
}

define LOG {
    LOG_FILE    = "${VARS_LOG_DIR}/app.log"
}
```

- PREFIX: use upper-case alphabet, number and underscore for convention.
- Value of item:  string, digit, boolean(`true`, `false`), functions and other variables that defined earlier.
- Can not refer other variables defined afterward.
- Multiple `define` blocks are processed in order.

### `module <moduleid>`

Defines the modules that implement interface `booter.Boot` to be called within the booter process.

The order in which they are initialized and Start() is called defaults to the order in which they are written to the file.
If you specify a different priority, Start() will be called in that order. Stop() is called in the reverse order.


The following values can be set in the `module` block

-  `name` This name specifies the target module when performing a dependency injection with `inject` from another module.

- `priority` Specifies the order in which modules should be started, as an integer value. Smaller values will be Start() first.

- `diabled` Disables the module from being defined but not created or started.

- `inject <target> <field|method> { }` Injects the module into a field in the target module.
  The time of injection is before all modules are instantiated and Start() is called.
  Therefore, when implementing Start() in the target module, the current instance of the module must have been created, but the order in which it has been started is a consideration.

  In `<field|method>`, you can specify a field from the target module or a setter method that takes one parameter.

- `config { }` defines the config object of the target module.

Inside the module definition, you can use the variables and predefined functions defined above with `define` to build your syntax.

```
module "my_project/module_a" {
    diabled = lower(VARS_IP_ADDR) == "127.0.0.1" ? true : false
    config {
        DebugMode      = VARS_DEBUG_MODE
        ListenAddress  = VARS_IP_ADDR 
        LogFilePath    = "${VARS_LOG_DIR}/my.log"
        HomePath       = env("HOME", "/home/my")
        Madatory       = envOrError("APP_VALUE")
    }
    inject "module_b" "ModB" {}
}

module "my_project/module_b" {
    name = "module_b"
}

```

#### Functions
- `env(name, default)`  Returns the  environment variable, or default if none exists. ex) `env("HOME", "/usr/home")`
- `envOrError(name)` Returns the environment variable, or raises an error and exits the booter. ex) `envOrError("APP_VALUE")`
- `flag(name, default)` Returns the command line argument, or default if none. ex) `flag("--log-dir", "./tmp")`
- `flagOrError(name)` Returns command-line argument, if none, an error is raised and the booter exits. ex)`flagOrError("--log-dir")`
- `pname()` Returns the pname specified when booter is run.
- `version()` Returns the value set by the application with `booter.SetVersionString()`.
- `arg(i, default)` Returns the i-th argument of the command-line arguments, excluding flags (beginning with '-'), or default if none.
- `argOrError(i)` Returns the i-th argument of the command line, excluding flags (beginning with '-'), or an error.
- `arglen()` Returns the number of command-line arguments.
- `userDir()` : Get user's home directory, On Linux and macOS, it returns the $HOME environment variable.
- `userConfDir()` : Get user's config directory, On Linux, it returns $XDG_CONFIG_HOME as specified by https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html if non-empty, else $HOME/.config. On macOS, it returns $HOME/Library/Application Support.
- `prefDir(subdir)`: Get $HOME/.config/$subdir
- `upper(str)`
- `lower(str)`
- `min(a, b)`
- `max(a, b)`
- `strlen(str)`
- `substr(str, offset, len)`

Applications can define additional functions with `booter.SetFunction(name, function.Function)` before calling `booter.Startup()`.

> Typically, applications refer to settings in "Defaults -> Environment Variables -> Configuration Files -> Command Line Arguments", which can be implemented as follows.
`MyPath = flag("--my-path", env("MY_PATH", "/home/me"))}`

### Define modules

In order to use your own modules in the booter's config, you need to register them in the booter's registry before the booter starts. This is usually done in the init() function.

- Register the module by calling `booter.Register(id, configFactory, instanceFactory)`.
- It takes three parameters
- `id` is the identifier for the module, which can be any string, but by convention is the go module path.
- `configFactory` is a `func() T` function that returns pointer `T` to the module's config object.
   The config object can be filled with default values before returning, so that defaults are applied even if they are not specified in the config file.
- The `instanceFactory` is created from the object returned by the `configFactory` with the function `func(T) (booter.Boot, error)`.
   The values in the `config` block of the configuration file are applied and then entered as arguments to `instanceFactory`.
   Based on these values, an instance of the module is created and returned, or an error is returned.
   As the return type of `instnaceFactory` indicates, the instance must implement the `booter.Boot` interface.
- `Booter.Boot` is an interface with two functions: `Start() error` and `Stop()`.

module example)

```go
package myserver

func init() {
    booter.Register("myproject/myserver",
    func()*Config{
        // Config factory: returns default config.
        return &Config {
            Host: "127.0.0.1",
            Port: 12345,
        }
    },
    func(c *Config)(booter.Boot, error) {
        // Instance factory: processes config block that is passed from booter,
        // The received config has been updated by booter according to the config file.
        // This function returns new instance of the module with the received config.
        return &server{
            conf: c,
        }, nil
    })
}
type Config struct {
    Host string
    Port int
}

type server struct {
    conf *Config
}

func (this *server) Start() error {
    return nil
}

func (this *server) Stop() {
}
```

### Define main()


Here's the main() of a booter application in its simplest form.

```go
func main() {
    booter.Startup()
    booter.WaitSignal()
    booter.Shutdown()
}
```

When an application starts the booter by calling `booter.Startup()`, 
- booter reads the config files, lists the module definitions, and finds the specified modules based on their IDs. 
- Finds the specified modules based on their IDs.
- In order, it calls the configFactory of that module to get the default config object,
- Update the config object with the fields specified in the `config` block.
- Pass the modified config object to the instanceFactory to create instances of the module.
- Perform dependency injection based on the values specified in the `inject` block.
- Call each module's `Start()` in sequence.

If the booter has successfully started the application's modules according to your settings, 
call the `booter.WaitSignal()` to wait for a termination signal.
The program control flow is blocked at `booter.WaitSignal()`.
To exit the booter from this state, call `booter.NotifySignal()` in a separate go routine, or if you enter the `^C` signal, 
`booter.WaitSignal()` is returned and the program is terminated normally via `booter.Shutdown()`.

#### Cutomize command line arguments

When `booter.Startup()` is run, it is run with the following default command line arguments.
At least one of `--config-dir` and `--config`, is required, and the other flags are optional.
- `--config-dir <dir>` config directory path
- `-c, --config <file>` a single file config
- `--pname <name>` process name
- `--pid <path>` pid file path
- `--bootlog <path>` boot log path
- `--d, -daemon` run process in background, daemonize
- `--help` print this message

> Additional flags required by other applications can be specified 
> in the configuration file with `flag()` as described in functions above.

If you want to change this flag to a different name, 
you can do so by calling `booter.SetFlag()` before `booter.Startup()`.

```go
func SetFlag(flagType BootFlagType, longflag, shortflag, defaultValue string)
```

The `BootFlagType` is

```go
const (
	ConfigDirFlag
	ConfigFileFlag
	PnameFlag
	PidFlag
	BootlogFlag
	DaemonFlag
	HelpFlag
)
```

For example, to change the default flag `--config` to `--config-file`, you would do the following. 
(If you don't want to use shortflag, just set it to the empty string "" instead of "c").

```go
booter.SetFlag(ConfigFileFlag, "config-file", "c", "./conf/default.hcl")
```
